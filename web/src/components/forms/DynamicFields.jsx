import React from "react";
import { ParameterField } from "./ParameterField";

export function DynamicFields({ parameters = [], values, onChange, browserTargets }) {
  return <div className="dynamic-fields">{parameters.map((parameter) => <ParameterField key={parameter.key} parameter={parameter} value={values[parameter.key]} values={values} onChange={(value) => onChange(parameter.key, value)} browserTargets={browserTargets} />)}</div>;
}
