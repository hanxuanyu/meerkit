import React from "react";
import * as SliderPrimitive from "@radix-ui/react-slider";
import { cn } from "../../lib/utils";

export const Slider = React.forwardRef(function Slider({ className, trackStyle, value = [], ...props }, ref) {
  return <SliderPrimitive.Root ref={ref} className={cn("slider-root relative flex w-full touch-none select-none items-center", className)} value={value} {...props}>
    <SliderPrimitive.Track className="slider-track relative h-2 w-full grow overflow-hidden rounded-sm bg-secondary" style={trackStyle}>
      <SliderPrimitive.Range className="slider-range absolute h-full bg-transparent" />
    </SliderPrimitive.Track>
    {value.map((_, index) => <SliderPrimitive.Thumb key={index} className="slider-thumb block size-4 rounded-sm border-2 border-background bg-foreground shadow focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50" aria-label={`区间游标 ${index + 1}`} />)}
  </SliderPrimitive.Root>;
});
