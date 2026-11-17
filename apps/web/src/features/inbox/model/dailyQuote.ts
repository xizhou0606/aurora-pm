const DAY_IN_MILLISECONDS = 86_400_000
const QUOTE_CYCLE_DAYS = 66
const COPRIME_STEPS = Object.freeze([
  1, 5, 7, 13, 17, 19, 23, 25, 29, 31,
  35, 37, 41, 43, 47, 49, 53, 59, 61, 65,
])

function positiveModulo(value: number, divisor: number): number {
  return ((value % divisor) + divisor) % divisor
}

function localCalendarDay(date: Date): number {
  return Math.floor(Date.UTC(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
  ) / DAY_IN_MILLISECONDS)
}

export function dailyQuoteIndex(date: Date = new Date(), userId = 0): number {
  const cycleDay = positiveModulo(localCalendarDay(date), QUOTE_CYCLE_DAYS)
  const salt = Math.imul(Math.max(0, Math.trunc(userId)), 2_654_435_761) >>> 0
  const step = COPRIME_STEPS[salt % COPRIME_STEPS.length]
  const offset = positiveModulo(salt ^ (salt >>> 16), QUOTE_CYCLE_DAYS)
  return positiveModulo(cycleDay * step + offset, QUOTE_CYCLE_DAYS)
}

export function dailyQuote(
  quotes: readonly string[],
  userId: number,
  date: Date = new Date(),
): string {
  if (quotes.length !== QUOTE_CYCLE_DAYS) {
    throw new Error(`Daily quote catalog must contain exactly ${QUOTE_CYCLE_DAYS} entries`)
  }
  return quotes[dailyQuoteIndex(date, userId)]
}
