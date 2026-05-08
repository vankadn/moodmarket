import { useEffect } from "react";
import { CashContext } from "../services/api";

interface Props {
  ctx: CashContext;
  onDismiss: () => void;
}

export function CashContextCard({ ctx, onDismiss }: Props) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, 5000);
    return () => clearTimeout(timer);
  }, [onDismiss]);

  return (
    <div
      onClick={onDismiss}
      style={{
        background: "#fff8f0",
        border: "1px solid #fcd9b0",
        borderRadius: "10px",
        padding: "14px 16px",
        marginBottom: "1.25rem",
        cursor: "pointer",
      }}
    >
      <div style={{ fontSize: "13px", color: "#444", lineHeight: 1.5 }}>
        {ctx.message}
      </div>
      <div style={{ fontSize: "11px", color: "#b45309", marginTop: "6px" }}>
        Tap to dismiss
      </div>
    </div>
  );
}
