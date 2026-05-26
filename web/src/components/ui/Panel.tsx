type PanelProps = {
  title?: string;
  subtitle?: string;
  action?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
};

export function Panel({ title, subtitle, action, className = "", children }: PanelProps) {
  return (
    <article className={`panel app-panel ${className}`.trim()}>
      {(title || action) && (
        <div className="app-panel__head">
          <div>
            {title && <h2>{title}</h2>}
            {subtitle && <p>{subtitle}</p>}
          </div>
          {action}
        </div>
      )}
      <div className="app-panel__body">{children}</div>
    </article>
  );
}
