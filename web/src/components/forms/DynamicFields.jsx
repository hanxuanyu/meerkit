import React from "react";
import { getParameters } from "../../lib/parameterSchema";
import { ParameterField } from "./ParameterField";

export function DynamicFields({ descriptor, parameters, properties, values, onChange }) {
  const fields = parameters || getParameters(descriptor || { config_schema: { properties } });
  return <div className="dynamic-fields">{fields.map((parameter) => <ParameterField key={parameter.key} parameter={parameter} value={values[parameter.key]} values={values} onChange={(value) => onChange(parameter.key, value)} />)}</div>;
}
