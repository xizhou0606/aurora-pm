import { copy } from '@/shared/copy'
import { landingTeamMarks } from '../utils/story'

export function LandingUsedBy() {
  const teams = copy.landing.usedByTeams.map((name, index) => ({
    name,
    mark: landingTeamMarks[index],
  }))
  return (
    <section aria-label={copy.landing.usedByLabel} className="landing-usedby">
      <p className="landing-usedby-kicker">{copy.landing.usedByLabel}</p>
      <div className="landing-usedby-viewport">
        <div className="landing-usedby-track">
          {[0, 1].map((loop) => (
            <ul aria-hidden={loop === 1 ? true : undefined} className="landing-usedby-list" key={loop}>
              {teams.map((team) => (
                <li className="landing-usedby-item" key={`${loop}-${team.name}`}>
                  <img alt="" className="landing-usedby-mark" src={team.mark} />
                  <span>{team.name}</span>
                </li>
              ))}
            </ul>
          ))}
        </div>
      </div>
    </section>
  )
}
