import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { EmptyTableRow } from "./EmptyTableRow";

test("空表格行标记整行状态并跨越所有列", () => {
  render(
    <table>
      <tbody>
        <EmptyTableRow colSpan={5} />
      </tbody>
    </table>,
  );

  const description = screen.getByText("无记录");
  const cell = description.closest("td");
  if (cell === null) {
    throw new Error("missing empty table cell");
  }
  expect(cell).toHaveAttribute("colspan", "5");
  expect(cell.closest("tr")).toHaveClass("data-table__empty-row");
  expect(description.closest(".ant-empty")).toBeInTheDocument();
});
