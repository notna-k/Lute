import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const repo = 'https://github.com/notna-k/Lute';

export default defineConfig({
	site: 'https://notna-k.github.io',
	base: '/Lute',
	trailingSlash: 'always',
	// The API reference reads ../api/openapi.yaml.
	vite: { server: { fs: { allow: ['..'] } } },
	integrations: [
		starlight({
			title: 'Lute',
			description: 'A self-hosted CI server: jobs as YAML in Git, typed run forms, rootless workers.',
			logo: { light: './src/assets/logo-light.svg', dark: './src/assets/logo-dark.svg', alt: 'Lute' },
			favicon: '/favicon.svg',
			social: [{ icon: 'github', label: 'GitHub', href: repo }],
			editLink: { baseUrl: `${repo}/edit/master/site/` },
			customCss: ['./src/styles/theme.css'],
			components: { SiteTitle: './src/components/SiteTitle.astro' },
			expressiveCode: {
				themes: ['github-dark-dimmed', 'github-light'],
				styleOverrides: { borderRadius: '0', codeFontFamily: "'Geist Mono', ui-monospace, monospace" },
			},
			sidebar: [
				{
					label: 'Getting started',
					items: [
						{ label: 'Introduction', link: '/docs/' },
						{ slug: 'docs/quick-start' },
						{ slug: 'docs/how-it-works' },
					],
				},
				{
					label: 'Jobs',
					items: [{ slug: 'docs/jobs/definitions' }, { slug: 'docs/jobs/parameters' }, { slug: 'docs/jobs/git-sync' }],
				},
				{ label: 'Workers', items: [{ slug: 'docs/workers/running' }] },
				{ label: 'Operations', items: [{ slug: 'docs/configuration' }] },
				{
					label: 'API',
					items: [
						{ slug: 'docs/api' },
						{ slug: 'docs/api/webhooks' },
						{ label: 'Runs', link: '/docs/api/runs/' },
						{ label: 'Workers', link: '/docs/api/workers/' },
						{ label: 'Worker commands', link: '/docs/api/commands/' },
						{ label: 'Registration tokens', link: '/docs/api/tokens/' },
					],
				},
			],
		}),
	],
});
