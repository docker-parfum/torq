import ReactSelect, { Props, components } from "react-select";
import {
  ChevronDown16Regular as ChevronDownIcon,
  WarningRegular as WarningIcon,
  ErrorCircleRegular as ErrorIcon,
} from "@fluentui/react-icons";
import styles from "./select.module.scss";
import classNames from "classnames";
import { GetColorClass, GetSizeClass, InputColorVaraint, InputSizeVariant } from "../variants";
import { useId } from "react";

export type SelectOptionType = { value: string; label: string };

const customStyles = {
  indicatorSeparator: () => {
    return {};
  },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  control: (provided: any, state: any) => ({
    ...provided,
    borderRadius: 4,
    backgroundColor: state.isFocused ? "var(--input-focus-background)" : "var(--input-default-background)",
    borderColor: state.isFocused ? "var(--input-focus-border-color)" : "transparent",
    boxShadow: "none",
    "&:hover": {
      backgroundColor: state.isFocused ? "var(--input-focus-background)" : "var(--input-hover-background)",
      boxShadow: "none",
    },
    fontSize: "var(--input-font-size)",
    height: "var(--input-height)",
    minHeight: "var(--input-height)",
  }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  placeholder: (provided: any) => {
    return {
      ...provided,
      color: "var(--input-placeholder-color)",
      fontSize: "var(--input-font-size)",
    };
  },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  dropdownIndicator: (provided: any, _: any) => ({
    ...provided,
    color: "var(--input-color)",
    fontSize: "var(--input-font-size)",
    padding: "var(--indicator-padding)",
  }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  singleValue: (provided: any) => ({
    ...provided,
    color: "var(--input-color)",
    fontSize: "var(--input-font-size)",
  }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  input: (provided: any) => ({
    ...provided,
    margin: "0",
    padding: "0",
  }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  option: (provided: any, state: any) => ({
    ...provided,
    color: "var(--input-color)",
    fontSize: "var(--input-font-size)",
    background: state.isFocused ? "var(--input-default-background)" : "#ffffff",
    "&:hover": {
      boxShadow: "none",
      backgroundColor: "var(--input-hover-background)",
    },
    borderRadius: "4px",
  }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  menuList: (provided: any, _: any) => ({
    ...provided,
    borderColor: "transparent",
    boxShadow: "none",
    padding: "8px",
    display: "flex",
    flexDirection: "column",
    gap: "4px",
  }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  menu: (provided: any, _: any) => ({
    ...provided,
    margin: "8px 4px",
    clip: "initial",
    width: "100%",
    borderColor: "transparent",
    borderRadius: "4px",
    boxShadow: "var(--hover-box-shadow)",
    zIndex: "10",
  }),
};

export type SelectProps = Props & {
  label?: string;
  colorVariant?: InputColorVaraint;
  sizeVariant?: InputSizeVariant;
  warningText?: string;
  errorText?: string;
};

export default function Select({
  label,
  colorVariant,
  sizeVariant,
  warningText,
  errorText,
  ...selectProps
}: SelectProps) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const DropdownIndicator = (props: any) => {
    return (
      <components.DropdownIndicator {...props}>
        <ChevronDownIcon />
      </components.DropdownIndicator>
    );
  };
  const inputId = useId();
  let inputColorClass = GetColorClass(colorVariant);
  if (warningText != undefined) {
    inputColorClass = GetColorClass(InputColorVaraint.warning);
  }
  if (errorText != undefined) {
    inputColorClass = GetColorClass(InputColorVaraint.error);
  }
  if (selectProps.isDisabled === true) {
    inputColorClass = GetColorClass(InputColorVaraint.disabled);
  }

  return (
    <div className={classNames(styles.inputWrapper, GetSizeClass(sizeVariant), inputColorClass)}>
      {label && (
        <div className={styles.labelWrapper}>
          <label htmlFor={selectProps.id || inputId} className={styles.label}>
            {label}
          </label>
        </div>
      )}
      <ReactSelect
        id={selectProps.id || inputId}
        components={{ DropdownIndicator }}
        className={selectProps.className}
        styles={customStyles}
        {...selectProps}
      />
      {errorText && (
        <div className={classNames(styles.feedbackWrapper, styles.feedbackError)}>
          <div className={styles.feedbackIcon}>
            <ErrorIcon />
          </div>
          <div className={styles.feedbackText}>{errorText}</div>
        </div>
      )}
      {warningText && (
        <div className={classNames(styles.feedbackWrapper, styles.feedbackWarning)}>
          <div className={styles.feedbackIcon}>
            <WarningIcon />
          </div>
          <div className={styles.feedbackText}>{warningText}</div>
        </div>
      )}
    </div>
  );
}
