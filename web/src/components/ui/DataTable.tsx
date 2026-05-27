import { EmptyTableRow } from "./EmptyTableRow";

type DataTableProps<Row extends Record<string, React.ReactNode>> = {
  columns: Array<{
    key: keyof Row;
    label: string;
  }>;
  rows: Row[];
};

export function DataTable<Row extends Record<string, React.ReactNode>>({
  columns,
  rows,
}: DataTableProps<Row>) {
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={String(column.key)} scope="col">
                {column.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 && <EmptyTableRow colSpan={columns.length} />}
          {rows.map((row, index) => (
            <tr key={index}>
              {columns.map((column) => (
                <td key={String(column.key)}>{row[column.key]}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
