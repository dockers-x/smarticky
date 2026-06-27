import { describe, expect, it } from "vitest";
import type { Note } from "../api/types";
import {
  activeCalendarFilter,
  activityCountsByDate,
  activityIntensity,
  buildMonthCalendar,
  monthKeysForYear,
  shiftMonth,
} from "./noteCalendar";

function note(
  id: string,
  created_at: string,
  updated_at: string,
): Note {
  return {
    id,
    title: id,
    content: "",
    color: "",
    protection_mode: "none",
    content_redacted: false,
    is_starred: false,
    is_deleted: false,
    folder_id: null,
    tags: [],
    created_at,
    updated_at,
  };
}

describe("note calendar helpers", () => {
  it("counts activity by selected basis and timezone", () => {
    const notes = [
      note("a", "2026-06-21T16:30:00Z", "2026-06-22T08:00:00Z"),
      note("b", "2026-06-22T03:00:00Z", "2026-06-22T09:00:00Z"),
    ];

    expect(activityCountsByDate(notes, "created", "Asia/Shanghai")).toEqual({
      "2026-06-22": 2,
    });
    expect(activityCountsByDate(notes, "updated", "UTC")).toEqual({
      "2026-06-22": 2,
    });
  });

  it("builds a full-week month matrix with clickable in-month empty days", () => {
    const cells = buildMonthCalendar(
      "2026-06",
      { "2026-06-22": 3 },
      "2026-06-22",
      "2026-06-05",
    );

    expect(cells).toHaveLength(35);
    expect(cells[0]).toMatchObject({
      date: "2026-05-31",
      isCurrentMonth: false,
    });
    expect(cells.find((cell) => cell.date === "2026-06-05")).toMatchObject({
      count: 0,
      isCurrentMonth: true,
      isSelected: true,
    });
    expect(cells.find((cell) => cell.date === "2026-06-22")).toMatchObject({
      count: 3,
      isToday: true,
      intensity: 4,
    });
  });

  it("calculates stable activity intensity buckets", () => {
    expect(activityIntensity(0, 8)).toBe(0);
    expect(activityIntensity(1, 8)).toBe(1);
    expect(activityIntensity(3, 8)).toBe(2);
    expect(activityIntensity(5, 8)).toBe(3);
    expect(activityIntensity(7, 8)).toBe(4);
  });

  it("detects the active single-day calendar filter", () => {
    expect(
      activeCalendarFilter({
        createdFrom: "",
        createdTo: "",
        updatedFrom: "2026-06-22",
        updatedTo: "2026-06-22",
      }),
    ).toEqual({ basis: "updated", date: "2026-06-22" });

    expect(
      activeCalendarFilter({
        createdFrom: "2026-06-22",
        createdTo: "2026-06-23",
        updatedFrom: "",
        updatedTo: "",
      }),
    ).toBeNull();
  });

  it("navigates month keys without external date libraries", () => {
    expect(shiftMonth("2026-01", -1)).toBe("2025-12");
    expect(shiftMonth("2026-12", 1)).toBe("2027-01");
    expect(monthKeysForYear(2026).slice(0, 3)).toEqual([
      "2026-01",
      "2026-02",
      "2026-03",
    ]);
  });
});
