type EmptyTableRowProps = {
  colSpan: number;
  label?: string;
};

export function EmptyTableRow({ colSpan, label = "无记录" }: EmptyTableRowProps) {
  return (
    <tr className="data-table__empty-row">
      <td className="data-table__empty" colSpan={colSpan}>
        {label}
      </td>
    </tr>
  );
}
