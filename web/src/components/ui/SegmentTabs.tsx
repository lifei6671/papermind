type SegmentTabsProps = {
  items: string[];
  active?: string;
  ariaLabel?: string;
  className?: string;
  onChange?: (item: string) => void;
};

export function SegmentTabs({
  items,
  active = items[0],
  ariaLabel,
  className = "",
  onChange,
}: SegmentTabsProps) {
  return (
    <div aria-label={ariaLabel} className={`segment-tabs ${className}`.trim()} role="tablist">
      {items.map((item) => {
        const isActive = item === active;

        return (
          <button
            aria-selected={isActive}
            className={isActive ? "segment-tabs__item segment-tabs__item--active" : "segment-tabs__item"}
            key={item}
            onClick={() => onChange?.(item)}
            role="tab"
            type="button"
          >
            {item}
          </button>
        );
      })}
    </div>
  );
}
