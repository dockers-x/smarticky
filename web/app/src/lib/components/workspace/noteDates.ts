import { dateKey } from "../../calendar/noteCalendar";
import { t, type Language } from "../../stores/preferences";

function calendar(language: Language, timeZone: string, today: string) {
  const locale = language === "zh" ? "zh-CN" : "en-US";
  const format = (options: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat(locale, { timeZone, ...options });
  const key = (date: Date) => dateKey(date, timeZone);
  const time = format({ hour: "numeric", minute: "2-digit" });
  const monthDay = format({ month: "long", day: "numeric" });
  const full = format({ year: "numeric", month: "long", day: "numeric" });
  const shortMonth = language === "zh" ? "numeric" : "short";
  const compact = format({ month: shortMonth, day: "numeric" });
  const compactYear = format({ year: "numeric", month: shortMonth, day: "numeric" });
  const yesterday = new Date(Date.parse(today + "T00:00:00Z") - 86400000).toISOString().slice(0, 10);
  return {
    key,
    groupLabel(day: string, date: Date) {
      if (day === today) return t("today", language);
      if (day === yesterday) return t("yesterday", language);
      return (day.slice(0, 4) === today.slice(0, 4) ? monthDay : full).format(date);
    },
    short(date: Date) {
      const day = key(date);
      if (day === today) return time.format(date);
      return (day.slice(0, 4) === today.slice(0, 4) ? compact : compactYear).format(date);
    },
  };
}

let cached: { language: Language; zone: string; day: string; dates: ReturnType<typeof calendar> } | undefined;
export function noteDates(language: Language, zone: string) {
  const now = new Date();
  const day = dateKey(now, zone);
  if (cached?.language === language && cached.zone === zone && cached.day === day) return cached.dates;
  const dates = calendar(language, zone, day);
  cached = { language, zone, day, dates };
  return dates;
}
