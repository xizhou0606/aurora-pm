import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi, type Mock } from 'bun:test'

import { copy } from '@/shared/copy'

import { fetchReleaseNotes } from '../api/release-notes'
import { ReleaseNotesPage } from './ReleaseNotesPage'

vi.mock('../api/release-notes', () => ({
  fetchReleaseNotes: vi.fn(),
  RELEASE_NOTES_ENDPOINT: '/api/release-notes',
}))

const fetchReleaseNotesMock = fetchReleaseNotes as unknown as Mock<typeof fetchReleaseNotes>

const ENTRIES = [
  {
    version: '1.1.0',
    date: '2026-09-03',
    summary: ['摘要一', '摘要二'],
    sections: [{ title: '新增', items: ['新增一'] }],
  },
  { version: '1.0.0', date: '2026-08-01', summary: [], sections: [] },
]

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <ReleaseNotesPage />
    </QueryClientProvider>,
  )
}

// 发版流程服务端化后,"当前版本"= 最新一期公告,页头徽章与其一致;
// 因此版本号会同时出现在页头徽章与文章标题两处。
describe('ReleaseNotesPage', () => {
  afterEach(() => {
    delete window.__RUNTIME_CONFIG__
    vi.clearAllMocks()
  })

  it('lists every release with summary, sections, and dates', async () => {
    fetchReleaseNotesMock.mockResolvedValue(ENTRIES)
    renderPage()

    expect(
      (await screen.findByRole('heading', { level: 1 })).textContent,
    ).toBe(copy.releaseNotes.title)
    expect((await screen.findAllByText('v1.1.0')).length).toBeGreaterThan(0)
    expect(screen.getByText('v1.0.0')).toBeTruthy()
    expect(screen.getByText('2026-09-03')).toBeTruthy()
    expect(screen.getByText('摘要一')).toBeTruthy()
    expect(screen.getByText('摘要二')).toBeTruthy()
    expect(screen.getByText('新增一')).toBeTruthy()
    // 最新一期公告即当前版本,不再单独标"最新"。
    expect(screen.getByText(copy.releaseNotes.currentVersion)).toBeTruthy()
    expect(screen.queryByText(copy.releaseNotes.latest)).toBeNull()
  })

  it('ignores the stale baked build version once announcements are served', async () => {
    fetchReleaseNotesMock.mockResolvedValue(ENTRIES)
    window.__RUNTIME_CONFIG__ = { PM_APP_VERSION: '1.0.0' }
    renderPage()

    expect(await screen.findByText(copy.releaseNotes.currentVersion)).toBeTruthy()
    expect(screen.queryByText(copy.releaseNotes.latest)).toBeNull()
  })

  it('falls back to the shared empty state without release data', async () => {
    fetchReleaseNotesMock.mockResolvedValue([])
    renderPage()

    expect(await screen.findByText(copy.releaseNotes.emptyTitle)).toBeTruthy()
    expect(screen.queryByText('v1.1.0')).toBeNull()
  })

  it('links the brand lockup back to the home page', async () => {
    fetchReleaseNotesMock.mockResolvedValue(ENTRIES)
    renderPage()

    await screen.findAllByText('v1.1.0')
    const homeLink = screen.getByRole('link', { name: copy.releaseNotes.backToHome })
    expect(homeLink.getAttribute('href')).toBe('/')
  })

  it('shows the loading skeleton while the request is pending', () => {
    fetchReleaseNotesMock.mockReturnValue(new Promise<never>(() => {}))
    renderPage()

    expect(screen.getByRole('status')).toBeTruthy()
    expect(screen.queryByText('v1.1.0')).toBeNull()
  })

  it('shows the failure banner with retry and recovers after refetch', async () => {
    fetchReleaseNotesMock.mockRejectedValueOnce(new Error('network down'))
    renderPage()

    const banner = await screen.findByRole('alert')
    expect(banner.textContent).toContain(copy.errors.genericDescription)
    expect(screen.queryByText('v1.1.0')).toBeNull()

    fetchReleaseNotesMock.mockResolvedValue(ENTRIES)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.retry }))

    await waitFor(() => {
      expect(screen.queryAllByText('v1.1.0').length).toBeGreaterThan(0)
    })
  })
})
