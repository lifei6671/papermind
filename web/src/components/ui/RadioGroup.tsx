import type { InputHTMLAttributes, ReactNode } from "react";
import { createContext, useContext, useId } from "react";

type RadioGroupContextValue = {
  name: string;
  onValueChange: (value: string) => void;
  value: string;
};

const RadioGroupContext = createContext<RadioGroupContextValue | null>(null);

type RadioGroupProps = {
  ariaLabel: string;
  children: ReactNode;
  className?: string;
  name?: string;
  onValueChange: (value: string) => void;
  value: string;
};

export function RadioGroup({ ariaLabel, children, className, name, onValueChange, value }: RadioGroupProps) {
  const generatedName = useId();

  return (
    <RadioGroupContext.Provider value={{ name: name ?? generatedName, onValueChange, value }}>
      <div aria-label={ariaLabel} className={className} role="radiogroup">
        {children}
      </div>
    </RadioGroupContext.Provider>
  );
}

type RadioGroupItemProps = Omit<InputHTMLAttributes<HTMLInputElement>, "checked" | "name" | "onChange" | "type"> & {
  value: string;
};

export function RadioGroupItem({ className = "", id, value, ...props }: RadioGroupItemProps) {
  const context = useContext(RadioGroupContext);
  const generatedID = useId();

  if (!context) {
    throw new Error("RadioGroupItem must be used inside RadioGroup");
  }

  return (
    <input
      checked={context.value === value}
      className={["ui-radio-group-item", className].filter(Boolean).join(" ")}
      id={id ?? generatedID}
      name={context.name}
      onChange={() => context.onValueChange(value)}
      type="radio"
      value={value}
      {...props}
    />
  );
}
