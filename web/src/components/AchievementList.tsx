import type { Achievement } from '../api'
import { EmptyState } from './States'

export default function AchievementList({ achievements }: { achievements: Achievement[] }) {
  if (achievements.length === 0) {
    return <EmptyState>The server listed no achievements for this account.</EmptyState>
  }

  return (
    <ul className="achievements">
      {achievements.map((achievement) => {
        const spoiler = achievement.hidden && !achievement.achieved
        const name = spoiler ? 'Hidden achievement' : (achievement.display_name || achievement.id)
        const description = spoiler ? '' : achievement.description
        const unlocked = achievement.achieved ? new Date(achievement.unlock_time * 1000) : null
        const validTime = unlocked !== null && !Number.isNaN(unlocked.getTime())

        return (
          <li className="achievement" key={`${achievement.appid}:${achievement.id}`}>
            <div className="achievement__head">
              <span className="achievement__name">{name}</span>
              <span className={`achievement__state${achievement.achieved ? ' is-earned' : ''}`}>
                {achievement.achieved ? 'Unlocked' : 'Locked'}
              </span>
            </div>
            {description ? <p className="achievement__desc muted">{description}</p> : null}
            {validTime && unlocked ? (
              <p className="achievement__time">
                Unlocked <time dateTime={unlocked.toISOString()}>{unlocked.toLocaleString()}</time>
              </p>
            ) : null}
          </li>
        )
      })}
    </ul>
  )
}
