// The documentation navigation tree. This is the single source of truth for
// the sidebar, the docs index taxonomy, and the prev/next pager.
export const nav = [
  {
    label: 'Getting started',
    pages: [
      { slug: 'getting-started/install', title: 'Install' },
      { slug: 'getting-started/quick-start', title: 'Quick start' },
    ],
  },
  {
    label: 'Guides',
    pages: [
      { slug: 'guides/configuration', title: 'Configuration' },
      { slug: 'guides/interactive', title: 'Interactive mode' },
      { slug: 'guides/non-interactive', title: 'Non-interactive mode' },
      { slug: 'guides/commands', title: 'Slash commands' },
      { slug: 'guides/keyboard', title: 'Keyboard reference' },
    ],
  },
  {
    label: 'Concepts',
    pages: [
      { slug: 'concepts/architecture', title: 'Architecture' },
      { slug: 'concepts/pruner', title: 'The pruner' },
      { slug: 'concepts/trace-log', title: 'Trace log' },
      { slug: 'concepts/mcp-servers', title: 'MCP servers' },
    ],
  },
  {
    label: 'Playground',
    pages: [
      { slug: 'playground/overview', title: 'Overview' },
      { slug: 'playground/running-tasks', title: 'Running tasks' },
      { slug: 'playground/reading-traces', title: 'Reading traces' },
      { slug: 'playground/tasks', title: 'Tasks reference' },
    ],
  },
  {
    label: 'Reference',
    pages: [
      { slug: 'reference/cli', title: 'CLI' },
      { slug: 'reference/config-file', title: 'Config file' },
      { slug: 'reference/api-keys', title: 'API keys' },
    ],
  },
];

// Flatten into a single ordered list for prev/next paging.
export const flat = nav.flatMap((sec) => sec.pages);
