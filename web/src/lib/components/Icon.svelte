<script lang="ts">
	let { name, size = 20, stroke = 2 }: { name: string; size?: number; stroke?: number } = $props();

	const paths: Record<string, string> = {
		back: 'M15 5l-7 7 7 7',
		close: 'M6 6l12 12M18 6L6 18',
		plus: 'M12 5v14M5 12h14',
		check: 'M5 12l5 5 9-10',
		home: 'M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z',
		friends:
			'M9 4.5a3.5 3.5 0 1 1 0 7 3.5 3.5 0 0 1 0-7zM17 6.5a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5zM2.5 20c.8-3.5 3.4-5.5 6.5-5.5s5.7 2 6.5 5.5M15.5 14.6c2.6-.3 5 1.3 6 4.4',
		stats: 'M5 20V11M12 20V4M19 20v-6',
		user: 'M12 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8zM4 21c1-4 4.3-6 8-6s7 2 8 6',
		camera: 'M4 8h3l2-3h6l2 3h3v11H4zM12 9.5a3.5 3.5 0 1 1 0 7 3.5 3.5 0 0 1 0-7z',
		receipt: 'M6 3h12v18l-3-2-3 2-3-2-3 2zM9 8h6M9 12h6',
		clip: 'M21 12.5l-8.5 8.5a5 5 0 0 1-7-7L14 5.5a3.3 3.3 0 0 1 4.7 4.7L10.2 18.7a1.7 1.7 0 0 1-2.4-2.4L15.5 8.6',
		search: 'M11 4a7 7 0 1 1 0 14 7 7 0 0 1 0-14zM20 20l-4-4',
		qr: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h3v3M21 14v.01M14 21h3M21 18v3',
		clock: 'M12 3.5a8.5 8.5 0 1 1 0 17 8.5 8.5 0 0 1 0-17zM12 7.5V12l3 2',
		swap: 'M4 8h13l-3-3M20 16H7l3 3',
		copy: 'M9 9h11v11H9zM5 15H4V4h11v1',
		chevron: 'M9 5l7 7-7 7',
		block: 'M12 3.5a8.5 8.5 0 1 1 0 17 8.5 8.5 0 0 1 0-17zM6 6l12 12',
		share: 'M12 3v12M7 8l5-5 5 5M5 14v6h14v-6',
		trash: 'M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3',
		image: 'M3 5h18v14H3zM9 8.4a1.6 1.6 0 1 1 0 3.2 1.6 1.6 0 0 1 0-3.2zM21 16l-5-5-8 8',
		rotate: 'M20 12a8 8 0 1 1-2.34-5.66M20 4v5h-5',
		flip: 'M12 3v18M9 7L4 12l5 5zM15 7l5 5-5 5z'
	};
</script>

<svg
	aria-hidden="true"
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width={stroke}
	stroke-linecap="round"
	stroke-linejoin="round"
>
	<path d={paths[name] ?? ''} />
</svg>
