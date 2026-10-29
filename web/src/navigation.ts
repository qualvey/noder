export type TabKey = 'nodes' | 'users' | 'files' | 'help' | 'tools'

export const mainNavItems = [
  {
    key: 'nodes',
    to: '/nodes',
    labelKey: 'nav.nodes',
    icon: 'nodes',
  },
  {
    key: 'users',
    to: '/users',
    labelKey: 'nav.users',
    icon: 'users',
  },
  {
    key: 'files',
    to: '/files',
    labelKey: 'nav.files',
    icon: 'files',
  },
  {
    key: 'help',
    to: '/help',
    labelKey: 'nav.help',
    icon: 'help',
  },
  {
    key: 'tools',
    to: '/tools',
    labelKey: 'nav.tools',
    icon: 'tools',
  },
] as const
