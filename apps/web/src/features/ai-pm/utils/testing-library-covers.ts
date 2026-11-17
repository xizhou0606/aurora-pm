const CASE_COVERS = [
  '/testing-covers/case-mint.png',
  '/testing-covers/case-lilac.png',
  '/testing-covers/case-blue.png',
  '/testing-covers/case-sand.png',
  '/testing-covers/case-teal.png',
] as const

const REPORT_COVERS = [
  '/testing-covers/report-mint.png',
  '/testing-covers/report-lilac.png',
  '/testing-covers/report-blue.png',
  '/testing-covers/report-sand.png',
] as const

export function testingLibraryCoverSrc(kind: 'test_case' | 'test_report', id: number) {
  const covers = kind === 'test_report' ? REPORT_COVERS : CASE_COVERS
  return covers[Math.abs(id) % covers.length]
}
