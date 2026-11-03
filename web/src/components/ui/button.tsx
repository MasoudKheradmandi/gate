import { forwardRef, type ButtonHTMLAttributes } from "react";
import { Loader2 } from "lucide-react";
import { cn } from "../../lib/utils";

type Variant = "primary" | "secondary" | "ghost" | "danger" | "danger-ghost";
type Size = "sm" | "md" | "lg" | "icon";

const variants: Record<Variant, string> = {
  primary: "bg-brand-solid text-on-brand border border-transparent hover:brightness-90 dark:hover:brightness-110",
  secondary: "bg-transparent text-brand border border-brand hover:bg-brand/10",
  ghost: "text-fg-muted hover:bg-surface-3 hover:text-fg border border-transparent",
  danger: "bg-danger text-white border border-transparent hover:brightness-90 dark:text-[#151515]",
  "danger-ghost": "text-danger border border-danger hover:bg-danger/10",
};
const sizes: Record<Size, string> = {
  sm: "h-7 px-2.5 text-xs gap-1.5 rounded-[3px]",
  md: "h-8 px-3.5 text-sm gap-2 rounded-[3px]",
  lg: "h-10 px-4 text-base gap-2 rounded-[3px]",
  icon: "h-8 w-8 rounded-[3px] justify-center",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant; size?: Size; loading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "secondary", size = "md", loading, disabled, className, children, type = "button", ...rest }, ref,
) {
  return (
    <button
      ref={ref} type={type} disabled={disabled || loading} aria-busy={loading || undefined}
      className={cn(
        "inline-flex shrink-0 select-none items-center whitespace-nowrap font-medium transition-colors duration-100 disabled:opacity-50",
        variants[variant], sizes[size], className,
      )}
      {...rest}
    >
      {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden />}
      {children}
    </button>
  );
});

export const IconButton = forwardRef<HTMLButtonElement, ButtonProps & { label: string }>(function IconButton(
  { label, variant = "ghost", size = "icon", className, ...rest }, ref,
) {
  return <Button ref={ref} variant={variant} size={size} aria-label={label} title={label} className={className} {...rest} />;
});
