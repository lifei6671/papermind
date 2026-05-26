import { Button } from "./Button";
import { useState } from "react";
import { CalendarDays, ChevronDown, ChevronLeft, ChevronRight, Clock3 } from "lucide-react";

export type DateRangePickerValue = {
  startDate: string;
  startTime: string;
  endDate: string;
  endTime: string;
  label?: string;
};

export type DateRangePickerPreset = DateRangePickerValue & {
  label: string;
};

type CalendarDay = {
  key: string;
  label: number;
  display: string;
};

type CalendarMonth = {
  key: string;
  label: string;
  leading: number;
  trailing: number;
  days: CalendarDay[];
};

type DateRangePickerProps = {
  defaultValue: DateRangePickerValue;
  label?: string;
  presets?: DateRangePickerPreset[];
  onChange?: (value: DateRangePickerValue) => void;
};

const weekDays = ["一", "二", "三", "四", "五", "六", "日"];

function parseISODate(date: string) {
  const [year, month, day] = date.split("-").map(Number);

  return new Date(year, month - 1, day || 1);
}

function formatMonthTitle(date: Date) {
  return `${date.getFullYear()}年${date.getMonth() + 1}月`;
}

function addMonths(date: Date, months: number) {
  return new Date(date.getFullYear(), date.getMonth() + months, 1);
}

function createCalendarMonth(date: Date): CalendarMonth {
  const year = date.getFullYear();
  const month = date.getMonth();
  const monthPrefix = `${year}-${String(month + 1).padStart(2, "0")}`;
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const leading = (new Date(year, month, 1).getDay() + 6) % 7;
  const trailing = 42 - leading - daysInMonth;

  return {
    key: monthPrefix,
    label: formatMonthTitle(date),
    leading,
    trailing,
    days: Array.from({ length: daysInMonth }, (_, index) => {
      const day = index + 1;
      const display = `${monthPrefix}-${String(day).padStart(2, "0")}`;

      return {
        key: display,
        label: day,
        display,
      };
    }),
  };
}

function isWithinRange(day: string, range: DateRangePickerValue) {
  return day >= range.startDate && day <= range.endDate;
}

function createCustomRange(startDate: string, endDate: string, current: DateRangePickerValue): DateRangePickerValue {
  const orderedStart = startDate <= endDate ? startDate : endDate;
  const orderedEnd = startDate <= endDate ? endDate : startDate;

  return {
    startDate: orderedStart,
    startTime: current.startTime,
    endDate: orderedEnd,
    endTime: current.endTime,
    label: buildRangeLabel(orderedStart, current.startTime, orderedEnd, current.endTime),
  };
}

function buildRangeLabel(startDate: string, startTime: string, endDate: string, endTime: string) {
  return startDate === endDate && startTime === endTime
    ? `${startDate} ${startTime}`
    : `${startDate} ${startTime} 至 ${endDate} ${endTime}`;
}

export function DateRangePicker({
  defaultValue,
  label = "按时间筛选",
  presets = [],
  onChange,
}: DateRangePickerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [confirmedRange, setConfirmedRange] = useState<DateRangePickerValue>(defaultValue);
  const [draftRange, setDraftRange] = useState<DateRangePickerValue>(defaultValue);
  const [pendingStart, setPendingStart] = useState<string | null>(null);
  const [visibleMonth, setVisibleMonth] = useState(() => parseISODate(defaultValue.startDate));
  const calendarMonths = [createCalendarMonth(visibleMonth), createCalendarMonth(addMonths(visibleMonth, 1))];
  const triggerLabel = confirmedRange.label || buildRangeLabel(
    confirmedRange.startDate,
    confirmedRange.startTime,
    confirmedRange.endDate,
    confirmedRange.endTime,
  );

  const selectDay = (day: string) => {
    if (!pendingStart) {
      setDraftRange(createCustomRange(day, day, draftRange));
      setPendingStart(day);
      return;
    }

    setDraftRange(createCustomRange(pendingStart, day, draftRange));
    setPendingStart(null);
  };

  const selectPreset = (preset: DateRangePickerPreset) => {
    setDraftRange(preset);
    setVisibleMonth(parseISODate(preset.startDate));
    setPendingStart(null);
  };

  const updateStartTime = (startTime: string) => {
    setDraftRange((current) => ({
      ...current,
      startTime,
      label: buildRangeLabel(current.startDate, startTime, current.endDate, current.endTime),
    }));
  };

  const updateEndTime = (endTime: string) => {
    setDraftRange((current) => ({
      ...current,
      endTime,
      label: buildRangeLabel(current.startDate, current.startTime, current.endDate, endTime),
    }));
  };

  const clearRange = () => {
    setDraftRange(defaultValue);
    setConfirmedRange(defaultValue);
    setVisibleMonth(parseISODate(defaultValue.startDate));
    setPendingStart(null);
    setIsOpen(false);
    onChange?.(defaultValue);
  };

  const confirmRange = () => {
    const nextRange = {
      ...draftRange,
      label: draftRange.label || buildRangeLabel(
        draftRange.startDate,
        draftRange.startTime,
        draftRange.endDate,
        draftRange.endTime,
      ),
    };

    setConfirmedRange(nextRange);
    setPendingStart(null);
    setIsOpen(false);
    onChange?.(nextRange);
  };

  return (
    <div className="soft-select usage-date-filter">
      <span>{label}</span>
      <Button
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        className="usage-date-filter__trigger"
        onClick={() => {
          setDraftRange(confirmedRange);
          setVisibleMonth(parseISODate(confirmedRange.startDate));
          setPendingStart(null);
          setIsOpen((current) => !current);
        }}
        type="button"
      >
        <CalendarDays aria-hidden="true" size={15} />
        {triggerLabel}
        <ChevronDown aria-hidden="true" size={14} />
      </Button>

      {isOpen && (
        <div aria-label="时间区间选择" className="date-range-popover" role="dialog">
          <div className="date-range-calendar">
            <Button
              aria-label="上个月"
              className="date-range-nav"
              onClick={() => setVisibleMonth((current) => addMonths(current, -1))}
              type="button"
            >
              <ChevronLeft aria-hidden="true" size={16} />
            </Button>
            <Button
              aria-label="下个月"
              className="date-range-nav date-range-nav--next"
              onClick={() => setVisibleMonth((current) => addMonths(current, 1))}
              type="button"
            >
              <ChevronRight aria-hidden="true" size={16} />
            </Button>

            {calendarMonths.map((month) => (
              <div className="date-range-month" key={month.key}>
                <h2>{month.label}</h2>
                <div className="date-range-weekdays">
                  {weekDays.map((day) => (
                    <span key={day}>{day}</span>
                  ))}
                </div>
                <div className="date-range-days">
                  {Array.from({ length: month.leading }).map((_, index) => (
                    <span aria-hidden="true" className="date-range-day date-range-day--empty" key={`${month.key}-empty-${index}`} />
                  ))}
                  {month.days.map((day) => {
                    const isRangeStart = day.display === draftRange.startDate;
                    const isRangeEnd = day.display === draftRange.endDate;
                    const isRangeMiddle = isWithinRange(day.display, draftRange) && !isRangeStart && !isRangeEnd;

                    return (
                      <Button
                        aria-label={day.display}
                        aria-pressed={isRangeStart || isRangeMiddle || isRangeEnd}
                        className={[
                          "date-range-day",
                          isRangeStart ? "date-range-day--start" : "",
                          isRangeMiddle ? "date-range-day--middle" : "",
                          isRangeEnd ? "date-range-day--end" : "",
                        ]
                          .filter(Boolean)
                          .join(" ")}
                        key={day.key}
                        onClick={() => selectDay(day.display)}
                        type="button"
                      >
                        {day.label}
                      </Button>
                    );
                  })}
                  {Array.from({ length: month.trailing }).map((_, index) => (
                    <span aria-hidden="true" className="date-range-day date-range-day--empty" key={`${month.key}-trailing-${index}`} />
                  ))}
                </div>
              </div>
            ))}
          </div>

          <div className="date-range-fields">
            <span className="date-range-field date-range-field--date">
              <CalendarDays aria-hidden="true" size={14} />
              {draftRange.startDate}
            </span>
            <label className="date-range-field">
              <Clock3 aria-hidden="true" size={14} />
              <input aria-label="开始时间" onChange={(event) => updateStartTime(event.target.value)} type="time" value={draftRange.startTime} />
            </label>
            <span className="date-range-field date-range-field--date">
              <CalendarDays aria-hidden="true" size={14} />
              {draftRange.endDate}
            </span>
            <label className="date-range-field">
              <Clock3 aria-hidden="true" size={14} />
              <input aria-label="结束时间" onChange={(event) => updateEndTime(event.target.value)} type="time" value={draftRange.endTime} />
            </label>
          </div>

          {presets.length > 0 && (
            <div className="date-range-presets">
              {presets.map((preset) => (
                <Button
                  className={preset.label === draftRange.label ? "date-range-preset date-range-preset--active" : "date-range-preset"}
                  key={preset.label}
                  onClick={() => selectPreset(preset)}
                  type="button"
                >
                  {preset.label}
                </Button>
              ))}
            </div>
          )}

          <div className="date-range-actions">
            <Button className="date-range-clear" onClick={clearRange} type="button">清除</Button>
            <Button className="date-range-confirm" onClick={confirmRange} type="button">
              确认查询
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
