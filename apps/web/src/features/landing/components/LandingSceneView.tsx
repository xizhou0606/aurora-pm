import { copy } from '@/shared/copy'
import { chapterPad } from '../utils/story'

export function LandingCopyPanel({
  index,
  workspaceHref,
}: {
  index: number
  workspaceHref: string
}) {
  const chapter = copy.landing.chapters[index]
  return (
    <div className="landing-scene-copy">
      <p className="landing-eyebrow" data-reveal="copy">{copy.landing.chapterKicker(chapterPad(index), chapter.rail)}</p>
      <h2 data-reveal="copy">{chapter.title}</h2>
      <p className="landing-copy-body" data-reveal="copy">{chapter.body}</p>
      <ul className="landing-pills">
        {chapter.items.map((item) => (
          <li data-reveal="copy" key={item}>{item}</li>
        ))}
      </ul>
      {chapter.cta ? (
        <a className="landing-primary" data-reveal="copy" href={workspaceHref}>
          {copy.landing.enterWorkspace}
        </a>
      ) : null}
    </div>
  )
}
