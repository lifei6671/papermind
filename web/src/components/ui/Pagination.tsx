import AntPagination from "antd/es/pagination";
import { Select } from "./Select";

type PaginationProps = {
  page: number;
  pageSize: number;
  pageSizeOptions?: number[];
  total: number;
  onPageChange: (page: number) => void;
  onPageSizeChange?: (pageSize: number) => void;
};

export function Pagination({ onPageChange, onPageSizeChange, page, pageSize, pageSizeOptions, total }: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const shouldShowPageSize = !!onPageSizeChange && !!pageSizeOptions?.length;

  return (
    <nav aria-label="分页" className="pagination pagination--right">
      {shouldShowPageSize && (
        <div className="pagination__page-size">
          <span>每页条数</span>
          <div className="pagination__page-size-select">
            <Select
              ariaLabel="每页条数"
              onChange={(value) => onPageSizeChange(Number(value))}
              options={pageSizeOptions.map((option) => ({
                value: String(option),
                label: `${option} 条 / 页`,
              }))}
              value={String(pageSize)}
            />
          </div>
        </div>
      )}
      <AntPagination
        className="pagination__controls"
        current={page}
        itemRender={(_, type, originalElement) => {
          if (type === "prev") {
            return <button type="button">上一页</button>;
          }
          if (type === "next") {
            return <button type="button">下一页</button>;
          }
          return originalElement;
        }}
        onChange={(nextPage) => onPageChange(nextPage)}
        pageSize={pageSize}
        showSizeChanger={false}
        showTotal={(value) => `共 ${value} 条`}
        total={total}
      />
      <strong className="pagination__current">
        第 {page} / {totalPages} 页
      </strong>
    </nav>
  );
}
