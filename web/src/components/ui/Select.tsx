import AntSelect from "antd/es/select";

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
  return (
    <AntSelect
      aria-label={ariaLabel}
      className="ui-select-trigger"
      classNames={{ popup: { root: "ui-select-content" } }}
      getPopupContainer={(trigger) => trigger.parentElement ?? document.body}
      onChange={(nextValue) => onChange(nextValue)}
      options={options}
      placeholder={placeholder}
      transitionName=""
      value={value}
      virtual={false}
    />
  );
}
