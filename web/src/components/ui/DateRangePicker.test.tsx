import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
  it("点击上一月和下一月时真实切换日历月份", async () => {
    const user = userEvent.setup();

    render(<DateRangePicker defaultValue={defaultValue} />);

    await user.click(screen.getByRole("button", { name: /最近48小时/ }));
    expect(screen.getByText("2026年5月")).toBeInTheDocument();
    expect(screen.getByText("2026年6月")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "上个月" }));
    expect(screen.getByText("2026年4月")).toBeInTheDocument();
    expect(screen.getByText("2026年5月")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "下个月" }));
    expect(screen.getByText("2026年6月")).toBeInTheDocument();
  });

  it("月份切换支持跨年", async () => {
    const user = userEvent.setup();

    render(
      <DateRangePicker
        defaultValue={{
          startDate: "2026-01-05",
          startTime: "09:30",
          endDate: "2026-01-08",
          endTime: "18:45",
          label: "自定义",
        }}
      />,
    );

    await user.click(screen.getByRole("button", { name: /自定义/ }));
    await user.click(screen.getByRole("button", { name: "上个月" }));

    expect(screen.getByText("2025年12月")).toBeInTheDocument();
    expect(screen.getByText("2026年1月")).toBeInTheDocument();
  });

  it("每个月固定渲染六行日期格以避免切换月份时跳动", async () => {
    const user = userEvent.setup();
    const { container } = render(<DateRangePicker defaultValue={defaultValue} />);

    await user.click(screen.getByRole("button", { name: /最近48小时/ }));

    const monthDayGrids = container.querySelectorAll(".date-range-days");

    expect(monthDayGrids).toHaveLength(2);
    monthDayGrids.forEach((grid) => {
      expect(grid.children).toHaveLength(42);
    });
  });

  it("修改起止时间并确认后触发 onChange", async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();

    render(<DateRangePicker defaultValue={defaultValue} onChange={handleChange} />);

    await user.click(screen.getByRole("button", { name: /最近48小时/ }));
    await user.clear(screen.getByLabelText("开始时间"));
    await user.type(screen.getByLabelText("开始时间"), "08:30");
    await user.clear(screen.getByLabelText("结束时间"));
    await user.type(screen.getByLabelText("结束时间"), "21:15");
    await user.click(screen.getByRole("button", { name: "确认查询" }));

    expect(handleChange).toHaveBeenCalledWith({
      startDate: "2026-05-06",
      startTime: "08:30",
      endDate: "2026-05-08",
      endTime: "21:15",
      label: "2026-05-06 08:30 至 2026-05-08 21:15",
    });
    expect(screen.getByRole("button", { name: /2026-05-06 08:30 至 2026-05-08 21:15/ })).toBeInTheDocument();
  });
});
