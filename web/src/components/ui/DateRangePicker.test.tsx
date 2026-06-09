import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { DateRangePicker, type DateRangePickerValue } from "./DateRangePicker";

const defaultValue: DateRangePickerValue = {
  startDate: "2026-05-06",
  startTime: "16:17",
  endDate: "2026-05-08",
  endTime: "16:17",
  label: "最近48小时",
};

describe("DateRangePicker", () => {
  it("使用 Ant Design RangePicker 展示默认时间区间", () => {
    const { container } = render(<DateRangePicker defaultValue={defaultValue} />);

    expect(screen.getByText("按时间筛选")).toBeInTheDocument();
    expect(container.querySelector(".ant-picker-range")).toBeInTheDocument();
    expect(container.querySelector("input[value='2026-05-06 16:17']")).toBeInTheDocument();
    expect(container.querySelector("input[value='2026-05-08 16:17']")).toBeInTheDocument();
  });

  it("修改时间区间后需要确认才触发 onChange", () => {
    const handleChange = vi.fn();
    const { container } = render(<DateRangePicker defaultValue={defaultValue} onChange={handleChange} />);
    const inputs = container.querySelectorAll("input");

    expect(inputs).toHaveLength(2);
    fireEvent.change(inputs[0], { target: { value: "2026-05-06 08:30" } });
    fireEvent.blur(inputs[0]);
    fireEvent.change(inputs[1], { target: { value: "2026-05-08 21:15" } });
    fireEvent.blur(inputs[1]);

    expect(handleChange).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "确认查询" }));

    expect(handleChange).toHaveBeenCalledWith({
      startDate: "2026-05-06",
      startTime: "08:30",
      endDate: "2026-05-08",
      endTime: "21:15",
      label: "2026-05-06 08:30 至 2026-05-08 21:15",
    });
  });

  it("确认查询时会归一化反向时间区间", () => {
    const handleChange = vi.fn();
    const { container } = render(<DateRangePicker defaultValue={defaultValue} onChange={handleChange} />);
    const inputs = container.querySelectorAll("input");

    fireEvent.change(inputs[0], { target: { value: "2026-05-09 10:00" } });
    fireEvent.blur(inputs[0]);
    fireEvent.change(inputs[1], { target: { value: "2026-05-08 09:00" } });
    fireEvent.blur(inputs[1]);
    fireEvent.click(screen.getByRole("button", { name: "确认查询" }));

    expect(handleChange).toHaveBeenCalledWith({
      startDate: "2026-05-08",
      startTime: "09:00",
      endDate: "2026-05-09",
      endTime: "10:00",
      label: "2026-05-08 09:00 至 2026-05-09 10:00",
    });
  });

  it("不允许通过清空按钮恢复旧查询范围", () => {
    const handleChange = vi.fn();
    const { container } = render(<DateRangePicker defaultValue={defaultValue} onChange={handleChange} />);

    expect(container.querySelector(".ant-picker-clear")).not.toBeInTheDocument();
  });
});
