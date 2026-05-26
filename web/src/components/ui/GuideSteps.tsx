import { ThumbsUp } from "lucide-react";
import { CodeTerminal } from "./CodeTerminal";

export type GuideStep = {
  title: string;
  description?: React.ReactNode;
  code?: string[];
  prompt?: string;
  tone?: "normal" | "success";
};

type GuideStepsProps = {
  title: string;
  steps: GuideStep[];
  supportLabel?: string;
};

export function GuideSteps({ title, steps, supportLabel = "有疑问? 技术支持" }: GuideStepsProps) {
  return (
    <article className="panel guide-steps-card">
      <div className="guide-steps-card__head">
        <h2>{title}</h2>
        <button className="secondary-button" type="button">
          <ThumbsUp aria-hidden="true" size={16} />
          {supportLabel}
        </button>
      </div>
      <ol className="guide-steps">
        {steps.map((step, index) => (
          <li className={step.tone === "success" ? "guide-step guide-step--success" : "guide-step"} key={step.title}>
            <span className="guide-step__index">{index + 1}</span>
            <div>
              <h3>{step.title}</h3>
              {step.description && <p>{step.description}</p>}
              {step.code && <CodeTerminal lines={step.code} prompt={step.prompt} />}
            </div>
          </li>
        ))}
      </ol>
    </article>
  );
}
