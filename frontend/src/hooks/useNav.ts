import { useNavigate, useLocation } from 'react-router-dom'

export type Page =
  | 'dashboard'
  | 'workspaces'
  | 'devices'
  | 'channels'
  | 'realtime'
  | 'query'
  | 'alerts'
  | 'notifications'
  | 'audit'
  | 'settings'

const PATH_TO_PAGE: Record<string, Page> = {
  '/':             'dashboard',
  '/workspaces':   'workspaces',
  '/devices':      'devices',
  '/channels':     'channels',
  '/realtime':     'realtime',
  '/query':        'query',
  '/alerts':       'alerts',
  '/notifications':'notifications',
  '/audit':        'audit',
  '/settings':     'settings',
}

const PAGE_TO_PATH: Record<Page, string> = {
  dashboard:     '/',
  workspaces:    '/workspaces',
  devices:       '/devices',
  channels:      '/channels',
  realtime:      '/realtime',
  query:         '/query',
  alerts:        '/alerts',
  notifications: '/notifications',
  audit:         '/audit',
  settings:      '/settings',
}

export function useNav() {
  const navigate = useNavigate()
  const { pathname } = useLocation()

  const page: Page = PATH_TO_PAGE[pathname] ?? 'dashboard'

  function setPage(p: Page) {
    navigate(PAGE_TO_PATH[p])
  }

  return { page, setPage }
}
