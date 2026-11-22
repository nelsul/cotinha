package main

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func sum(xs []int64) int64 {
	var t int64
	for _, x := range xs {
		t += x
	}
	return t
}

func TestSplitEqual(t *testing.T) {
	got, msg := splitShares(8000, "equal", make([]participantIn, 4))
	if msg != "" || sum(got) != 8000 || got[0] != 2000 || got[3] != 2000 {
		t.Fatalf("got %v %q", got, msg)
	}
	got, _ = splitShares(1000, "equal", make([]participantIn, 3))
	if sum(got) != 1000 || got[0] != 334 || got[2] != 333 {
		t.Fatalf("remainder not spread: %v", got)
	}
}

func TestSplitWeight(t *testing.T) {
	// The bar example: 5, 2, 2 and 3 beers share R$ 120.
	parts := []participantIn{{Value: 5}, {Value: 2}, {Value: 2}, {Value: 3}}
	got, msg := splitShares(12000, "weight", parts)
	if msg != "" || got[0] != 5000 || got[1] != 2000 || got[3] != 3000 {
		t.Fatalf("got %v %q", got, msg)
	}
	got, _ = splitShares(1000, "weight", []participantIn{{Value: 1}, {Value: 1}, {Value: 1}})
	if sum(got) != 1000 {
		t.Fatalf("weights lost cents: %v", got)
	}
}

func TestSplitAmount(t *testing.T) {
	if _, msg := splitShares(1000, "amount", []participantIn{{Value: 500}, {Value: 400}}); msg == "" {
		t.Fatal("accepted parts that do not add up")
	}
	got, msg := splitShares(1000, "amount", []participantIn{{Value: 600}, {Value: 400}})
	if msg != "" || got[0] != 600 {
		t.Fatalf("got %v %q", got, msg)
	}
}

func TestAllocate(t *testing.T) {
	shares := []Share{
		{CotinhaID: "a", Amount: 2000, Confirmed: 500},
		{CotinhaID: "b", Amount: 3000, Pending: 1000},
	}
	got := allocate(shares, 2500)
	if len(got) != 2 || got[0].Amount != 1500 || got[1].Amount != 1000 {
		t.Fatalf("got %+v", got)
	}
	if allocate(shares, 3501) != nil {
		t.Fatal("allocated more than payable")
	}
}

func TestPixPayloadCRC(t *testing.T) {
	p := pixPayload("123e4567-e12b-12d1-a456-426655440000", "Fulano de Tal", "COTINHA", 2000)
	body, crc := p[:len(p)-4], p[len(p)-4:]
	if crc16(body) != crc {
		t.Fatal("bad CRC")
	}
	if want := "5405" + "20.00"; !contains(p, want) {
		t.Fatalf("amount field missing in %s", p)
	}
	// Known value for the standard CRC-16/CCITT-FALSE check string.
	if crc16("123456789") != "29B1" {
		t.Fatalf("crc16 = %s", crc16("123456789"))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestOptimizeImage(t *testing.T) {
	// A big, noisy PNG must come out as a JPEG within the size limit.
	src := image.NewRGBA(image.Rect(0, 0, 3000, 2000))
	for i := range src.Pix {
		src.Pix[i] = uint8(i * 7 % 251)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	out, ctype, ext := optimizeImage(buf.Bytes(), "image/png", "png", photoSide)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || ctype != "image/jpeg" || ext != "jpg" || cfg.Width != photoSide || cfg.Height != 1066 {
		t.Fatalf("got %s %dx%d err=%v", ctype, cfg.Width, cfg.Height, err)
	}
	if len(out) >= buf.Len() {
		t.Fatalf("not smaller: %d >= %d", len(out), buf.Len())
	}
	// Something Go can't decode is stored as it is.
	heic := []byte("\x00\x00\x00\x18ftypheic....")
	if got, ct, _ := optimizeImage(heic, "image/heic", "heic", photoSide); ct != "image/heic" || len(got) != len(heic) {
		t.Fatal("undecodable image was changed")
	}
}

func TestPixKeys(t *testing.T) {
	if _, ok := normalizePixKey("cpf", "529.982.247-25"); !ok {
		t.Fatal("valid CPF rejected")
	}
	if _, ok := normalizePixKey("cpf", "111.111.111-11"); ok {
		t.Fatal("invalid CPF accepted")
	}
	if k, ok := normalizePixKey("phone", "(92) 99999-1234"); !ok || k != "+5592999991234" {
		t.Fatalf("phone: %s %v", k, ok)
	}
	if _, ok := normalizePixKey("cnpj", "11.222.333/0001-81"); !ok {
		t.Fatal("valid CNPJ rejected")
	}
}
