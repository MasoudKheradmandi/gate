import type { HTMLAttributes, ReactNode, TdHTMLAttributes, ThHTMLAttributes } from "react";
import { cn } from "../../lib/utils";

export function Table({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("overflow-x-auto", className)}><table className="w-full border-collapse text-sm">{children}</table></div>;
}
export const THead = ({ children }: { children: ReactNode }) => <thead className="border-b-2 border-line bg-surface">{children}</thead>;
export const Th = ({ className, ...p }: ThHTMLAttributes<HTMLTableCellElement>) =>
  <th scope="col" className={cn("whitespace-nowrap px-4 py-2.5 text-left text-sm font-semibold text-fg", className)} {...p} />;
export const Tr = ({ className, ...p }: HTMLAttributes<HTMLTableRowElement>) =>
  <tr className={cn("border-b border-line last:border-b-0 hover:bg-surface-2", className)} {...p} />;
export const Td = ({ className, ...p }: TdHTMLAttributes<HTMLTableCellElement>) =>
  <td className={cn("px-4 py-2.5 align-middle", className)} {...p} />;
