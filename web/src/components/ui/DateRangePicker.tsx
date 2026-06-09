import DatePicker from "antd/es/date-picker";
import dayjs from "dayjs";
import type { Dayjs } from "dayjs";
import { useMemo, useState } from "react";

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

type DateRangePickerProps = {
  defaultValue: DateRangePickerValue;
  label?: string;
  presets?: DateRangePickerPreset[];
  onChange?: (value: DateRangePickerValue) => void;
};

const dateTimeFormat = "YYYY-MM-DD HH:mm";
const serializedDateTimeFormat = "YYYY-MM-DDTHH:mm";

export function DateRangePicker({
  defaultValue,
  label = "按时间筛选",
  presets = [],
  onChange,
}: DateRangePickerProps) {
  const [draftRange, setDraftRange] = useState<DateRangePickerValue>(defaultValue);
  const antPresets = useMemo(
    () => presets.map((preset) => ({
      label: preset.label,
      value: [valueToDayjs(preset, "start"), valueToDayjs(preset, "end")] as [Dayjs, Dayjs],
    })),
    [presets],
  );

  const commitRange = () => {
    const nextRange = normalizeRangeOrder(draftRange);
    setDraftRange(nextRange);
    onChange?.(nextRange);
  };

  const updateRangePart = (part: "start" | "end", value: string) => {
    const nextDateTime = normalizeDateTimeInput(value);
    if (!nextDateTime) {
      return;
    }

    setDraftRange((current) => {
      const nextRange = part === "start"
        ? valueWithDateTime(current, "start", nextDateTime)
        : valueWithDateTime(current, "end", nextDateTime);

      return {
        ...nextRange,
        label: buildRangeLabel(nextRange),
      };
    });
  };

  return (
    <label className="soft-select usage-date-filter">
      <span>{label}</span>
      <DatePicker.RangePicker
        allowClear={false}
        className="date-range-picker"
        format={dateTimeFormat}
        inputReadOnly={false}
        onBlur={(event, info) => updateRangePart(info.range ?? "start", inputValue(event.currentTarget))}
        onChange={(value) => {
          if (!value?.[0] || !value[1]) {
            return;
          }

          const nextRange = valueFromDates(value[0].format(serializedDateTimeFormat), value[1].format(serializedDateTimeFormat));
          setDraftRange({
            ...nextRange,
            label: buildRangeLabel(nextRange),
          });
        }}
        presets={antPresets}
        showTime={{ format: "HH:mm" }}
        value={[valueToDayjs(draftRange, "start"), valueToDayjs(draftRange, "end")]}
      />
      <button className="date-range-confirm" onClick={commitRange} type="button">
        确认查询
      </button>
    </label>
  );
}

function valueToDayjs(value: DateRangePickerValue, part: "start" | "end") {
  const dateTime = part === "start"
    ? `${value.startDate}T${value.startTime}`
    : `${value.endDate}T${value.endTime}`;

  return dayjs(dateTime);
}

function valueWithDateTime(value: DateRangePickerValue, part: "start" | "end", dateTime: string): DateRangePickerValue {
  const [date, time] = dateTime.split("T");
  if (part === "start") {
    return {
      ...value,
      startDate: date,
      startTime: time,
    };
  }

  return {
    ...value,
    endDate: date,
    endTime: time,
  };
}

function valueFromDates(startValue: string, endValue: string): DateRangePickerValue {
  const baseValue: DateRangePickerValue = {
    startDate: "",
    startTime: "",
    endDate: "",
    endTime: "",
  };

  return valueWithDateTime(valueWithDateTime(baseValue, "start", startValue), "end", endValue);
}

function buildRangeLabel(value: DateRangePickerValue) {
  const startLabel = `${value.startDate} ${value.startTime}`;
  const endLabel = `${value.endDate} ${value.endTime}`;
  return startLabel === endLabel ? startLabel : `${startLabel} 至 ${endLabel}`;
}

function normalizeRangeOrder(value: DateRangePickerValue) {
  const startValue = `${value.startDate}T${value.startTime}`;
  const endValue = `${value.endDate}T${value.endTime}`;
  const ordered = startValue <= endValue
    ? value
    : valueFromDates(endValue, startValue);
  return {
    ...ordered,
    label: buildRangeLabel(ordered),
  };
}

function normalizeDateTimeInput(value: string) {
  const parsedValue = dayjs(value.trim().replace(" ", "T"));
  return parsedValue.isValid() ? parsedValue.format(serializedDateTimeFormat) : "";
}

function inputValue(target: EventTarget & HTMLElement) {
  return target instanceof HTMLInputElement ? target.value : "";
}
