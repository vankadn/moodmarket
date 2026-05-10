import { useEffect, useRef, useState } from "react";
import { TradeReceipt, getOrderStatus } from "../services/api";

const statusColor: Record<string, string> = {
  filled:           "#1D9E75",
  pending_new:      "#888",
  accepted:         "#888",
  partially_filled: "#E6873A",
  canceled:         "#C0392B",
  expired:          "#C0392B",
  rejected:         "#C0392B",
};

const TERMINAL = new Set(["filled", "canceled", "expired", "rejected", "replaced"]);
const POLL_MS = 3000;
const MAX_ATTEMPTS = 20;       // ~60 s before giving up
const ACCEPTED_THRESHOLD = 5;  // 5 consecutive accepted polls → after-hours assumed

interface Props {
  receipts: TradeReceipt[];
  decisionId: string;
  onDone: () => void;
}

export function ReceiptScreen({ receipts: initial, decisionId, onDone }: Props) {
  const [receipts, setReceipts] = useState<TradeReceipt[]>(initial);
  // Per-order stop messages — presence means polling stopped for that order.
  const [stopNotes, setStopNotes] = useState<Record<string, string>>({});

  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Refs are readable inside the poll closure without stale values.
  const attemptsRef = useRef<Record<string, number>>({});
  const acceptedStreakRef = useRef<Record<string, number>>({});
  const stoppedRef = useRef<Set<string>>(new Set());

  const allSettled = receipts.every(
    (r) => TERMINAL.has(r.status) || r.order_id in stopNotes
  );
  const activelyPolling = !allSettled && receipts.some(
    (r) => !TERMINAL.has(r.status) && !(r.order_id in stopNotes)
  );

  useEffect(() => {
    if (allSettled) return;

    function poll() {
      // Exclude orders that are already terminal or explicitly stopped.
      const pending = receipts.filter(
        (r) => !TERMINAL.has(r.status) && !stoppedRef.current.has(r.order_id)
      );
      if (pending.length === 0) return;

      Promise.allSettled(pending.map((r) => getOrderStatus(r.order_id))).then((results) => {
        const newStops: Array<[string, string]> = [];

        setReceipts((prev) => {
          const updated = [...prev];
          results.forEach((result, i) => {
            const orderId = pending[i].order_id;

            const attempts = (attemptsRef.current[orderId] ?? 0) + 1;
            attemptsRef.current[orderId] = attempts;

            if (result.status === "fulfilled") {
              const idx = updated.findIndex((r) => r.order_id === orderId);
              if (idx !== -1) updated[idx] = result.value;

              const status = result.value.status.toLowerCase();
              if (status === "accepted") {
                const streak = (acceptedStreakRef.current[orderId] ?? 0) + 1;
                acceptedStreakRef.current[orderId] = streak;
                if (streak >= ACCEPTED_THRESHOLD && !stoppedRef.current.has(orderId)) {
                  newStops.push([orderId, "Orders accepted. Will fill when market opens (Mon–Fri 9:30am–4pm ET)."]);
                }
              } else {
                acceptedStreakRef.current[orderId] = 0;
              }
            }

            // Max-attempts check — only if not already stopped by after-hours logic above.
            if (
              attempts >= MAX_ATTEMPTS &&
              !stoppedRef.current.has(orderId) &&
              !newStops.some(([id]) => id === orderId)
            ) {
              newStops.push([orderId, "Market may be closed — check back when market opens."]);
            }
          });
          return updated;
        });

        if (newStops.length > 0) {
          newStops.forEach(([id]) => stoppedRef.current.add(id));
          setStopNotes((prev) => {
            const next = { ...prev };
            newStops.forEach(([id, note]) => { next[id] = note; });
            return next;
          });
        }

        // Schedule next poll after results land, not before — prevents pile-up on slow connections.
        timerRef.current = setTimeout(poll, POLL_MS);
      });
    }

    timerRef.current = setTimeout(poll, POLL_MS);
    return () => { if (timerRef.current) clearTimeout(timerRef.current); };
  }, [allSettled]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div style={{ background: "white", border: "1px solid #e0e0e0", borderRadius: "12px", padding: "1.25rem 1.5rem" }}>
      <div style={{ marginBottom: "1.25rem" }}>
        <div style={{ fontWeight: 600, fontSize: "15px", color: "#111" }}>
          {receipts.length > 0 ? "Orders placed" : "No orders placed"}
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: "8px", marginTop: "4px" }}>
          <span style={{ fontSize: "11px", color: "#bbb", fontFamily: "monospace" }}>
            decision {decisionId}
          </span>
          {activelyPolling && (
            <span style={{ fontSize: "11px", color: "#aaa" }}>· polling for fill…</span>
          )}
        </div>
      </div>

      {receipts.length === 0 ? (
        <p style={{ fontSize: "13px", color: "#999" }}>All orders failed — check logs for details.</p>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: "10px", marginBottom: "1.25rem" }}>
          {receipts.map((r) => (
            <div key={r.order_id}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 12px", background: "#f8f8f8", borderRadius: "8px" }}>
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
              {stopNotes[r.order_id] && (
                <div style={{ fontSize: "11px", color: "#888", padding: "6px 12px 0" }}>
                  {stopNotes[r.order_id]}
                </div>
              )}
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
