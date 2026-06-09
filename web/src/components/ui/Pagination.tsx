import AntPagination from "antd/es/pagination";

type PaginationProps = {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
};

export function Pagination({ page, pageSize, total, onPageChange }: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <nav aria-label="分页" className="pagination pagination--right">
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
