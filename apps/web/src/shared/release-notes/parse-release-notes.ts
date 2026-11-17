export type ReleaseNoteSection = {
  title: string
  items: string[]
}

export type ReleaseNoteEntry = {
  version: string
  date: string
  summary: string[]
  sections: ReleaseNoteSection[]
}

const VERSION_HEADING = /^##\s+\[([^\]]+)\](?:\s*-\s*(\d{4}-\d{2}-\d{2}))?[^\n]*/gm
const SECTION_HEADING = /^###\s+(.+?)[ \t]*$/gm
const BULLET_ITEM = /^[-*]\s+(.+)$/gm
const SUMMARY_TITLE = '更新摘要'

/**
 * Parse the constrained CHANGELOG.md format documented in its 维护约定:
 * `## [version] - YYYY-MM-DD` blocks whose `### 更新摘要` bullets become the
 * release summary and whose remaining `###` sections keep their title and
 * bullets. Entries come back newest first, matching the file order.
 */
export function parseReleaseNotes(markdown: string): ReleaseNoteEntry[] {
  const headings = [...markdown.matchAll(VERSION_HEADING)]
  return headings.map((heading, index) => {
    const start = (heading.index ?? 0) + heading[0].length
    const end = index + 1 < headings.length
      ? headings[index + 1].index ?? markdown.length
      : markdown.length
    return {
      version: heading[1].trim(),
      date: heading[2] ?? '',
      ...parseEntryBody(markdown.slice(start, end)),
    }
  })
}

/**
 * Parse a single release note block body (`contentMd`) as stored server side:
 * the same constrained format as one `## [version]` entry of CHANGELOG.md
 * without its version heading. `### 更新摘要` bullets become the summary and
 * remaining `###` sections keep their title and bullets.
 */
export function parseReleaseNoteContent(
  contentMd: string,
): { summary: string[]; sections: ReleaseNoteSection[] } {
  return parseEntryBody(contentMd)
}

function parseEntryBody(body: string): { summary: string[]; sections: ReleaseNoteSection[] } {
  const summary: string[] = []
  const sections: ReleaseNoteSection[] = []
  const headings = [...body.matchAll(SECTION_HEADING)]
  headings.forEach((heading, index) => {
    const start = (heading.index ?? 0) + heading[0].length
    const end = index + 1 < headings.length
      ? headings[index + 1].index ?? body.length
      : body.length
    const items = [...body.slice(start, end).matchAll(BULLET_ITEM)]
      .map((item) => item[1].trim())
      .filter(Boolean)
    const title = heading[1].trim()
    if (title === SUMMARY_TITLE) {
      summary.push(...items)
    } else if (items.length) {
      sections.push({ title, items })
    }
  })
  return { summary, sections }
}
