import AntSelect from "antd/es/select";
import { useId } from "react";

export type SelectOption = {
  value: string;
  label: string;
};

type SelectProps = {
  ariaLabel: string;
  options: SelectOption[];
  placeholder?: string;
  value?: string;
  onChange: (value: string) => void;
};

export function Select({ ariaLabel, onChange, options, placeholder, value }: SelectProps) {
  const selectID = useId();

  return (
    <AntSelect
      aria-label={ariaLabel}
      className="ui-select-trigger"
      classNames={{ popup: { root: "ui-select-content" } }}
      getPopupContainer={(trigger) => trigger.parentElement ?? document.body}
      id={selectID}
      onChange={(nextValue) => onChange(nextValue)}
      options={options}
      placeholder={placeholder}
      transitionName=""
      value={value}
      virtual={false}
    />
  );
}
