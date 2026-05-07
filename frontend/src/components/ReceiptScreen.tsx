import { TradeReceipt } from "../services/api";

const statusColor: Record<string, string> = {
  filled: "#1D9E75",
  pending_new: "#888",
  accepted: "#888",
  partially_filled: "#E6873A",
  canceled: "#C0392B",
};

interface Props {
  receipts: TradeReceipt[];
  decisionId: string;
  onDone: () => void;
}

export function ReceiptScreen({ receipts, decisionId, onDone }: Props) {
  return (
    <div style={{ background: "white", border: "1px solid #e0e0e0", borderRadius: "12px", padding: "1.25rem 1.5rem" }}>
      <div style={{ marginBottom: "1.25rem" }}>
        <div style={{ fontWeight: 600, fontSize: "15px", color: "#111" }}>
          {receipts.length > 0 ? "Orders placed" : "No orders placed"}
        </div>
        <div style={{ fontSize: "11px", color: "#bbb", marginTop: "4px", fontFamily: "monospace" }}>
          decision {decisionId}
        </div>
      </div>

      {receipts.length === 0 ? (
        <p style={{ fontSize: "13px", color: "#999" }}>All orders failed — check logs for details.</p>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: "10px", marginBottom: "1.25rem" }}>
          {receipts.map((r) => (
            <div key={r.order_id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 12px", background: "#f8f8f8", borderRadius: "8px" }}>
              <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
                <span style={{ background: "#f0f0f0", padding: "2px 8px", borderRadius: "6px", fontSize: "12px", fontWeight: 500 }}>
                  {r.ticker}
                </span>
                <span style={{ fontSize: "11px", color: statusColor[r.status] ?? "#888", fontWeight: 500, textTransform: "uppercase", letterSpacing: "0.04em" }}>
                  {r.status.replace(/_/g, " ")}
                </span>
              </div>
              <div style={{ textAlign: "right" }}>
                {r.filled_amount > 0 && (
                  <div style={{ fontSize: "14px", fontWeight: 500 }}>${r.filled_amount.toFixed(2)}</div>
                )}
                {r.filled_price > 0 && (
                  <div style={{ fontSize: "11px", color: "#999" }}>@ ${r.filled_price.toFixed(2)}</div>
                )}
                <div style={{ fontSize: "10px", color: "#ccc", fontFamily: "monospace", marginTop: "2px" }}>
                  {r.order_id.slice(0, 8)}…
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      <button
        onClick={onDone}
        style={{
          width: "100%", padding: "12px",
          background: "#f5f5f5", color: "#333",
          border: "none", borderRadius: "8px",
          fontSize: "14px", fontWeight: 500, cursor: "pointer",
        }}
      >
        Done
      </button>
    </div>
  );
}
