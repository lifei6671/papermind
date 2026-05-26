import { Button } from "./Button";
type PaginationProps = {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
};

export function Pagination({ page, pageSize, total, onPageChange }: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const canGoPrevious = page > 1;
  const canGoNext = page < totalPages;

  return (
    <nav aria-label="分页" className="pagination">
      <span>共 {total} 条</span>
      <div className="pagination__controls">
        <Button disabled={!canGoPrevious} onClick={() => onPageChange(page - 1)} type="button">
          上一页
        </Button>
        <strong>
          第 {page} / {totalPages} 页
        </strong>
        <Button disabled={!canGoNext} onClick={() => onPageChange(page + 1)} type="button">
          下一页
        </Button>
      </div>
    </nav>
  );
}
