import { beforeEach, describe, expect, it } from 'vitest'
import { i18n } from '@/lib/i18n'

/**
 * Hard-coded list mirrors services/user-service/internal/achievements/registry.go.
 * Keeping it here (for now) lets us verify every key the backend will ever
 * publish has a translation in both en and es. When Phase C moves definitions
 * to a backend endpoint, this test fetches the list from there instead.
 */
const REGISTRY: {
  key: string
  tiers: number
  flatDescription: boolean
}[] = [
  { key: 'games_played', tiers: 5, flatDescription: false },
  { key: 'games_won', tiers: 4, flatDescription: false },
  { key: 'win_streak', tiers: 3, flatDescription: false },
  { key: 'first_draw', tiers: 1, flatDescription: true },
  { key: 'ttt_perfect_game', tiers: 1, flatDescription: true },
  { key: 'ttt_games_played', tiers: 3, flatDescription: false },
]

describe('achievements i18n coverage', () => {
  for (const lang of ['en', 'es'] as const) {
    describe(`locale=${lang}`, () => {
      beforeEach(async () => {
        await i18n.changeLanguage(lang)
      })

      for (const def of REGISTRY) {
        it(`${def.key}: name and every tier resolves to a non-empty string`, () => {
          const name = i18n.t(`achievements.${def.key}.name`)
          expect(name, `missing name in ${lang}`).not.toBe(`achievements.${def.key}.name`)
          expect(name.length).toBeGreaterThan(0)

          if (def.flatDescription) {
            const desc = i18n.t(`achievements.${def.key}.description`)
            expect(desc, `missing description in ${lang}`).not.toBe(
              `achievements.${def.key}.description`,
            )
            expect(desc.length).toBeGreaterThan(0)
          }

          for (let tier = 1; tier <= def.tiers; tier++) {
            const tierName = i18n.t(`achievements.${def.key}.tiers.${tier}.name`)
            const tierDesc = i18n.t(`achievements.${def.key}.tiers.${tier}.description`, {
              threshold: 42,
            })
            expect(tierName, `missing ${def.key} tier ${tier} name in ${lang}`).not.toBe(
              `achievements.${def.key}.tiers.${tier}.name`,
            )
            expect(tierDesc, `missing ${def.key} tier ${tier} description in ${lang}`).not.toBe(
              `achievements.${def.key}.tiers.${tier}.description`,
            )
            expect(tierName.length).toBeGreaterThan(0)
            expect(tierDesc.length).toBeGreaterThan(0)
          }
        })
      }
    })
  }
})
