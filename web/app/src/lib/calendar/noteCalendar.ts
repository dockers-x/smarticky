import type { Note } from "../api/types";

export type CalendarTimeBasis = "updated" | "created";

export interface CalendarDateFilters {
  createdFrom: string;
  createdTo: string;
  updatedFrom: string;
  updatedTo: string;
}

export interface CalendarDayCell {
  date: string;
  label: number;
  count: number;
  intensity: number;
  isCurrentMonth: boolean;
  isToday: boolean;
  isSelected: boolean;
}

export interface ActiveCalendarFilter {
  basis: CalendarTimeBasis;
  date: string;
}

const daysInWeek = 7;

export function dateKey(date: Date, timeZone: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(date);
  const value = (type: string) =>
    parts.find((part) => part.type === type)?.value ?? "";
  return `${value("year")}-${value("month")}-${value("day")}`;
}

export function monthKey(date: Date, timeZone: string): string {
  return dateKey(date, timeZone).slice(0, 7);
}

export function todayKey(timeZone: string): string {
  return dateKey(new Date(), timeZone);
}

export function shiftMonth(month: string, delta: number): string {
  const parsed = parseMonth(month);
  const date = new Date(parsed.year, parsed.monthIndex + delta, 1);
  return formatMonth(date.getFullYear(), date.getMonth());
}

export function monthLabel(
  month: string,
  language: "zh" | "en",
): string {
  const parsed = parseMonth(month);
  return new Intl.DateTimeFormat(language === "zh" ? "zh-CN" : "en-US", {
    year: "numeric",
    month: "long",
  }).format(new Date(parsed.year, parsed.monthIndex, 1));
}

export function monthKeysForYear(year: number): string[] {
  return Array.from({ length: 12 }, (_, index) => formatMonth(year, index));
}

export function activityCountsByDate(
  notes: Note[],
  basis: CalendarTimeBasis,
  timeZone: string,
): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const note of notes) {
    const raw = basis === "created" ? note.created_at : note.updated_at;
    const key = dateKey(new Date(raw), timeZone);
    counts[key] = (counts[key] ?? 0) + 1;
  }
  return counts;
}

export function maxActivityCount(counts: Record<string, number>): number {
  return Math.max(1, ...Object.values(counts));
}

export function activityIntensity(count: number, maxCount: number): number {
  if (count <= 0) return 0;
  const ratio = count / Math.max(1, maxCount);
  if (ratio > 0.75) return 4;
  if (ratio > 0.5) return 3;
  if (ratio > 0.25) return 2;
  return 1;
}

export function buildMonthCalendar(
  month: string,
  counts: Record<string, number>,
  currentDate: string,
  selectedDate = "",
): CalendarDayCell[] {
  const parsed = parseMonth(month);
  const monthStart = new Date(parsed.year, parsed.monthIndex, 1);
  const monthEnd = new Date(parsed.year, parsed.monthIndex + 1, 0);
  const start = new Date(monthStart);
  start.setDate(monthStart.getDate() - monthStart.getDay());
  const end = new Date(monthEnd);
  end.setDate(monthEnd.getDate() + (daysInWeek - 1 - monthEnd.getDay()));

  const maxCount = maxActivityCount(counts);
  const cells: CalendarDayCell[] = [];
  for (
    const cursor = new Date(start);
    cursor <= end;
    cursor.setDate(cursor.getDate() + 1)
  ) {
    const key = formatDate(cursor.getFullYear(), cursor.getMonth(), cursor.getDate());
    const count = counts[key] ?? 0;
    cells.push({
      date: key,
      label: cursor.getDate(),
      count,
      intensity: activityIntensity(count, maxCount),
      isCurrentMonth:
        cursor.getFullYear() === parsed.year &&
        cursor.getMonth() === parsed.monthIndex,
      isToday: key === currentDate,
      isSelected: key === selectedDate,
    });
  }
  return cells;
}

export function activeCalendarFilter(
  filters: CalendarDateFilters,
): ActiveCalendarFilter | null {
  if (
    filters.updatedFrom &&
    filters.updatedFrom === filters.updatedTo
  ) {
    return { basis: "updated", date: filters.updatedFrom };
  }
  if (
    filters.createdFrom &&
    filters.createdFrom === filters.createdTo
  ) {
    return { basis: "created", date: filters.createdFrom };
  }
  return null;
}

function parseMonth(month: string): { year: number; monthIndex: number } {
  const [yearRaw, monthRaw] = month.split("-");
  const year = Number(yearRaw);
  const monthNumber = Number(monthRaw);
  if (!Number.isInteger(year) || !Number.isInteger(monthNumber)) {
    throw new Error(`Invalid month: ${month}`);
  }
  if (monthNumber < 1 || monthNumber > 12) {
    throw new Error(`Invalid month: ${month}`);
  }
  return { year, monthIndex: monthNumber - 1 };
}

function formatMonth(year: number, monthIndex: number): string {
  return `${year}-${String(monthIndex + 1).padStart(2, "0")}`;
}

function formatDate(year: number, monthIndex: number, day: number): string {
  return `${year}-${String(monthIndex + 1).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}
