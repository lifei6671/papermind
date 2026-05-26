import type { LucideIcon } from "lucide-react";

type GuideCalloutProps = {
  icon: LucideIcon;
  title: string;
  children: React.ReactNode;
  tone?: "success" | "warning" | "danger" | "neutral";
};

export function GuideCallout({ icon: Icon, title, children, tone = "neutral" }: GuideCalloutProps) {
  return (
    <section className={`guide-callout guide-callout--${tone}`}>
      <span className="guide-callout__icon">
        <Icon aria-hidden="true" size={18} />
      </span>
      <div>
        <h2>{title}</h2>
        <p>{children}</p>
      </div>
    </section>
  );
}
