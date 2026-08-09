// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://atakang7.github.io',
  base: '/cortex',
  integrations: [
    starlight({
      title: 'Cortex',
      description: 'Terminal coding agent built on Axon.',
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/atakang7/cortex' }],
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        { label: 'Start', items: [{ label: 'Lightning Quickstart', slug: '' }] },
      ],
    }),
  ],
});
