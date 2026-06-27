<script lang="ts">
  import {
    CalendarDays,
    ChevronLeft,
    ChevronRight,
    RotateCcw,
    X,
  } from "@lucide/svelte";
  import { onMount } from "svelte";
  import {
    activeCalendarFilter,
    activityCountsByDate,
    buildMonthCalendar,
    monthKey,
    monthKeysForYear,
    monthLabel,
    shiftMonth,
    todayKey,
    type CalendarDayCell,
    type CalendarTimeBasis,
  } from "../../calendar/noteCalendar";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";

  let calendarBasis: CalendarTimeBasis = "updated";
  let visibleMonth = "";
  let lastActiveFilterKey = "";
  let yearOverviewOpen = false;

  $: activeFilter = activeCalendarFilter($notesStore.searchFilters);
  $: activeFilterKey = activeFilter
    ? `${activeFilter.basis}:${activeFilter.date}`
    : "";
  $: selectedDate = activeFilter?.basis === calendarBasis ? activeFilter.date : "";
  $: if (activeFilter && activeFilter.basis !== calendarBasis) {
    calendarBasis = activeFilter.basis;
  }
  $: if (!activeFilterKey && lastActiveFilterKey) {
    lastActiveFilterKey = "";
  }
  $: if (activeFilter?.date && activeFilterKey !== lastActiveFilterKey) {
    lastActiveFilterKey = activeFilterKey;
    visibleMonth = activeFilter.date.slice(0, 7);
  }
  $: if (!visibleMonth) {
    visibleMonth = monthKey(new Date(), $preferencesStore.timeZone);
  }
  $: counts = activityCountsByDate(
    $notesStore.calendarNotes,
    calendarBasis,
    $preferencesStore.timeZone,
  );
  $: currentDate = todayKey($preferencesStore.timeZone);
  $: days = buildMonthCalendar(visibleMonth, counts, currentDate, selectedDate);
  $: visibleYear = Number(visibleMonth.slice(0, 4)) || new Date().getFullYear();
  $: yearCards = monthKeysForYear(visibleYear).map((month) => ({
    month,
    label: monthLabel(month, $preferencesStore.language),
    days: buildMonthCalendar(month, counts, currentDate, selectedDate),
  }));
  $: weekLabels =
    $preferencesStore.language === "zh"
      ? ["日", "一", "二", "三", "四", "五", "六"]
      : ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

  onMount(() => {
    visibleMonth =
      activeFilter?.date.slice(0, 7) ??
      monthKey(new Date(), $preferencesStore.timeZone);
    void notesStore.loadCalendarNotes();
  });

  function previousMonth(): void {
    visibleMonth = shiftMonth(visibleMonth, -1);
    yearOverviewOpen = false;
  }

  function nextMonth(): void {
    visibleMonth = shiftMonth(visibleMonth, 1);
    yearOverviewOpen = false;
  }

  function showToday(): void {
    visibleMonth = monthKey(new Date(), $preferencesStore.timeZone);
    yearOverviewOpen = false;
  }

  function selectMonth(month: string): void {
    visibleMonth = month;
    yearOverviewOpen = false;
  }

  function setBasis(basis: CalendarTimeBasis): void {
    calendarBasis = basis;
    if (activeFilter) {
      void notesStore.setCalendarDateFilter(activeFilter.date, basis);
    }
  }

  function selectDate(date: string): void {
    void notesStore.setCalendarDateFilter(date, calendarBasis);
  }

  function clearDateFilter(): void {
    void notesStore.clearCalendarDateFilter();
  }

  function cellLabel(day: CalendarDayCell): string {
    const basis =
      calendarBasis === "updated"
        ? t("calendarUpdatedBasis", $preferencesStore.language)
        : t("calendarCreatedBasis", $preferencesStore.language);
    if (day.count === 0) return `${day.date}, ${basis}`;
    return `${day.date}, ${day.count} ${t("notes", $preferencesStore.language)}, ${basis}`;
  }
</script>

<section
  class="note-calendar"
  aria-label={t("calendarView", $preferencesStore.language)}
>
  <div class="note-calendar__header">
    <button
      class="note-calendar__month"
      type="button"
      aria-expanded={yearOverviewOpen}
      on:click={() => (yearOverviewOpen = !yearOverviewOpen)}
    >
      <CalendarDays size={16} strokeWidth={1.8} aria-hidden="true" />
      <span>{monthLabel(visibleMonth, $preferencesStore.language)}</span>
    </button>

    <div class="note-calendar__nav" aria-label={t("calendarMonthNavigation", $preferencesStore.language)}>
      <button
        type="button"
        aria-label={t("previousMonth", $preferencesStore.language)}
        on:click={previousMonth}
      >
        <ChevronLeft size={15} strokeWidth={2} aria-hidden="true" />
      </button>
      <button
        type="button"
        aria-label={t("calendarToday", $preferencesStore.language)}
        on:click={showToday}
      >
        <RotateCcw size={14} strokeWidth={1.9} aria-hidden="true" />
      </button>
      <button
        type="button"
        aria-label={t("nextMonth", $preferencesStore.language)}
        on:click={nextMonth}
      >
        <ChevronRight size={15} strokeWidth={2} aria-hidden="true" />
      </button>
    </div>
  </div>

  <div class="note-calendar__controls">
    <div
      class="note-calendar__basis"
      role="group"
      aria-label={t("calendarTimeBasis", $preferencesStore.language)}
    >
      <button
        class:active={calendarBasis === "updated"}
        type="button"
        on:click={() => setBasis("updated")}
      >
        {t("calendarUpdatedBasis", $preferencesStore.language)}
      </button>
      <button
        class:active={calendarBasis === "created"}
        type="button"
        on:click={() => setBasis("created")}
      >
        {t("calendarCreatedBasis", $preferencesStore.language)}
      </button>
    </div>

    {#if activeFilter}
      <button
        class="note-calendar__clear"
        type="button"
        on:click={clearDateFilter}
      >
        <X size={13} strokeWidth={2} aria-hidden="true" />
        {activeFilter.date}
      </button>
    {/if}
  </div>

  {#if $notesStore.calendarError}
    <div class="note-calendar__message" role="alert">{$notesStore.calendarError}</div>
  {/if}

  {#if yearOverviewOpen}
    <div class="note-calendar-year" aria-label={t("calendarYearOverview", $preferencesStore.language)}>
      {#each yearCards as card (card.month)}
        <button
          class:active={card.month === visibleMonth}
          class="note-calendar-year__month"
          type="button"
          on:click={() => selectMonth(card.month)}
        >
          <span>{card.label}</span>
          <div aria-hidden="true">
            {#each card.days as day (day.date)}
              <i
                class:outside={!day.isCurrentMonth}
                class:selected={day.isSelected}
                class:today={day.isToday}
                data-level={day.intensity}
              ></i>
            {/each}
          </div>
        </button>
      {/each}
    </div>
  {/if}

  <div class:loading={$notesStore.calendarLoading} class="note-calendar-grid">
    {#each weekLabels as label}
      <span class="note-calendar-grid__weekday" aria-hidden="true">{label}</span>
    {/each}
    {#each days as day (day.date)}
      {#if day.isCurrentMonth}
        <button
          class:selected={day.isSelected}
          class:today={day.isToday}
          class="note-calendar-day"
          data-level={day.intensity}
          type="button"
          aria-current={day.isToday ? "date" : undefined}
          aria-label={cellLabel(day)}
          aria-pressed={day.isSelected}
          on:click={() => selectDate(day.date)}
        >
          <span>{day.label}</span>
          {#if day.count > 0}
            <small>{day.count}</small>
          {/if}
        </button>
      {:else}
        <span class="note-calendar-day outside">{day.label}</span>
      {/if}
    {/each}
  </div>
</section>
