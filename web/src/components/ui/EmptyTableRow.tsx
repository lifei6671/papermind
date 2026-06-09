import Empty from "antd/es/empty";

type EmptyTableRowProps = {
  colSpan: number;
  label?: string;
};

export function EmptyTableRow({ colSpan, label = "无记录" }: EmptyTableRowProps) {
  return (
    <tr className="data-table__empty-row">
      <td className="data-table__empty" colSpan={colSpan}>
        <Empty className="data-table__empty-state" description={label} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </td>
    </tr>
  );
}
