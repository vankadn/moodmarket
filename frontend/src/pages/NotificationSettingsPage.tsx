// src/pages/NotificationSettingsPage.tsx
import { useEffect, useState } from "react";
import { NotificationSettings, getNotificationSettings, updateNotificationSettings } from "../services/api";

interface Props {
  onBack: () => void;
  onSaved: () => void;
}

export function NotificationSettingsPage({ onBack, onSaved }: Props) {
  const [settings, setSettings] = useState<NotificationSettings>({ notification_email: "", phone: "" });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getNotificationSettings()
      .then(setSettings)
      .catch(() => setError("Failed to load notification settings"))
      .finally(() => setLoading(false));
  }, []);

  async function handleSave() {
    setSaving(true);
    setError(null);
    try {
      await updateNotificationSettings(settings);
      onSaved();
    } catch {
      setError("Failed to save — please try again");
    } finally {
      setSaving(false);
    }
  }

  const inputStyle: React.CSSProperties = {
    width: "100%", padding: "10px 12px", border: "1px solid #e0e0e0",
    borderRadius: "8px", fontSize: "14px", outline: "none",
    boxSizing: "border-box",
  };

  const labelStyle: React.CSSProperties = {
    fontSize: "11px", fontWeight: 600, color: "#aaa",
    letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "6px", display: "block",
  };

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "1.5rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#888", fontSize: "13px", cursor: "pointer", padding: 0 }}
        >
          ← Back
        </button>
        <h1 style={{ fontSize: "18px", fontWeight: 600, margin: 0, color: "#111" }}>Notifications</h1>
      </div>

      {loading ? (
        <div style={{ fontSize: "13px", color: "#888" }}>Loading…</div>
      ) : (
        <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1.25rem", display: "flex", flexDirection: "column", gap: "1rem" }}>
          <p style={{ fontSize: "13px", color: "#666", margin: "0 0 4px" }}>
            Get emailed or texted after each auto-invest run. Leave either field blank to skip that channel.
          </p>

          <div>
            <label style={labelStyle}>Email address</label>
            <input
              type="email"
              placeholder="you@example.com"
              value={settings.notification_email}
              onChange={(e) => setSettings(s => ({ ...s, notification_email: e.target.value }))}
              style={inputStyle}
            />
          </div>

          <div>
            <label style={labelStyle}>Phone (SMS)</label>
            <input
              type="tel"
              placeholder="+1 555 000 0000"
              value={settings.phone}
              onChange={(e) => setSettings(s => ({ ...s, phone: e.target.value }))}
              style={inputStyle}
            />
          </div>

          {error && (
            <div style={{ fontSize: "13px", color: "#c0392b", background: "#fdf0ee", padding: "8px 12px", borderRadius: "6px" }}>
              {error}
            </div>
          )}

          <button
            onClick={handleSave}
            disabled={saving}
            style={{
              padding: "10px 16px", background: saving ? "#ccc" : "#1a1a1a",
              color: "white", border: "none", borderRadius: "8px",
              fontSize: "14px", fontWeight: 500, cursor: saving ? "not-allowed" : "pointer",
            }}
          >
            {saving ? "Saving…" : "Save"}
          </button>
        </div>
      )}
    </div>
  );
}
