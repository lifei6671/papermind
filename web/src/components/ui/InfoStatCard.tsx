type InfoStatCardProps = {
  label: string;
  value: string;
  hint?: string;
  tone?: "default" | "accent" | "danger";
};

export function InfoStatCard({ label, value, hint, tone = "default" }: InfoStatCardProps) {
  return (
    <article className={`panel info-stat info-stat--${tone}`}>
      <span>{label}</span>
      <strong className="amount">{value}</strong>
      {hint && <small>{hint}</small>}
    </article>
  );
}
