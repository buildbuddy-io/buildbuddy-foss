import React from "react";
import Checkbox from "../checkbox/checkbox";
import { OutlinedButton } from "./button";

export type CheckboxButtonProps = JSX.IntrinsicElements["input"] & {
  checkboxOnLeft?: boolean;
  checkboxRef?: React.RefObject<HTMLInputElement>;
};

export default function CheckboxButton({
  children,
  checkboxOnLeft,
  checkboxRef,
  checked,
  onChange,
  className,
  ...props
}: CheckboxButtonProps) {
  const nodes = [
    <span>{children}</span>,
    <Checkbox ref={checkboxRef} checked={checked} onChange={onChange} {...props} />,
  ];

  return (
    <OutlinedButton className={`checkbox-button ${className || ""}`}>
      <label>{checkboxOnLeft ? nodes.reverse() : nodes}</label>
    </OutlinedButton>
  );
}
