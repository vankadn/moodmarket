import { useEffect, useRef, useState } from "react";
import {
  TaxDocument,
  DocumentType,
  listDocuments,
  uploadDocument,
  deleteDocument,
} from "../services/api";

interface Props {
  onBack: () => void;
}

const docTypeLabels: Record<DocumentType, string> = {
  w2: "W-2",
  "1099": "1099",
  "1098": "1098",
};

const docTypeDescriptions: Record<DocumentType, string> = {
  w2: "Wage & tax statement",
  "1099": "Freelance / investment income",
  "1098": "Mortgage interest statement",
};

function fmtDollar(val: string | undefined): string {
  if (!val) return "—";
  const num = parseFloat(val);
  if (!isNaN(num)) {
    return `$${num.toLocaleString("en-US", { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`;
  }
  return val;
}

function getKeyFields(doc: TaxDocument): { label: string; value: string }[] {
  switch (doc.DocumentType) {
    case "w2":
      return [
        { label: "Employer", value: doc.Fields.employer_name || "—" },
        { label: "Gross wages", value: fmtDollar(doc.Fields.gross_wages) },
        { label: "Fed withheld", value: fmtDollar(doc.Fields.federal_withheld) },
        { label: "State withheld", value: fmtDollar(doc.Fields.state_withheld) },
      ];
    case "1099":
      return [
        { label: "Payer", value: doc.Fields.payer_name || "—" },
        { label: "Gross income", value: fmtDollar(doc.Fields.gross_income) },
        { label: "Fed withheld", value: fmtDollar(doc.Fields.federal_withheld) },
        { label: "Type", value: (doc.Fields.income_type || "").toUpperCase() || "—" },
      ];
    case "1098":
      return [
        { label: "Lender", value: doc.Fields.lender_name || "—" },
        { label: "Interest paid", value: fmtDollar(doc.Fields.mortgage_interest_paid) },
        { label: "Principal", value: fmtDollar(doc.Fields.outstanding_principal) },
        { label: "Points paid", value: fmtDollar(doc.Fields.points_paid) },
      ];
    default:
      return [];
  }
}

function DocCard({ doc, onDelete }: { doc: TaxDocument; onDelete: () => void }) {
  const [confirming, setConfirming] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const keyFields = getKeyFields(doc);

  async function handleDelete() {
    if (!confirming) { setConfirming(true); return; }
    setDeleting(true);
    try {
      await deleteDocument(doc.ID);
      onDelete();
    } catch {
      setDeleting(false);
      setConfirming(false);
    }
  }

  return (
    <div style={{
      background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem",
      marginBottom: "12px",
    }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "10px" }}>
            <span style={{
              fontSize: "11px", fontWeight: 700, padding: "2px 8px", borderRadius: "4px",
              background: "#1a1a1a", color: "white", letterSpacing: "0.05em",
            }}>
              {docTypeLabels[doc.DocumentType]}
            </span>
            {doc.TaxYear > 0 && (
              <span style={{ fontSize: "13px", color: "#666" }}>Tax Year {doc.TaxYear}</span>
            )}
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "8px 16px" }}>
            {keyFields.map(({ label, value }) => (
              <div key={label}>
                <div style={{ fontSize: "11px", color: "#999", marginBottom: "1px" }}>{label}</div>
                <div style={{ fontSize: "13px", fontWeight: 500, color: "#222" }}>{value}</div>
              </div>
            ))}
          </div>
        </div>
        <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: "6px", marginLeft: "12px", flexShrink: 0 }}>
          <button
            onClick={handleDelete}
            disabled={deleting}
            style={{
              background: confirming ? "#c0392b" : "none",
              border: confirming ? "none" : "1px solid #e0e0e0",
              color: confirming ? "white" : "#999",
              borderRadius: "6px", padding: "4px 10px",
              fontSize: "12px", cursor: deleting ? "not-allowed" : "pointer",
            }}
          >
            {deleting ? "Deleting…" : confirming ? "Confirm delete" : "Delete"}
          </button>
          {confirming && !deleting && (
            <button
              onClick={() => setConfirming(false)}
              style={{ background: "none", border: "none", color: "#999", fontSize: "12px", cursor: "pointer", padding: 0 }}
            >
              Cancel
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

export function Documents({ onBack }: Props) {
  const [docs, setDocs] = useState<TaxDocument[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedType, setSelectedType] = useState<DocumentType>("w2");
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    listDocuments()
      .then(data => setDocs(data))
      .catch(() => setDocs([]))
      .finally(() => setLoading(false));
  }, []);

  async function handleUpload() {
    if (!file) return;
    setUploading(true);
    setUploadError(null);
    try {
      const doc = await uploadDocument(file, selectedType);
      setDocs(prev => [doc, ...prev]);
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
    } catch (e: unknown) {
      setUploadError(e instanceof Error ? e.message : "Upload failed");
    } finally {
      setUploading(false);
    }
  }

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "1.5rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: 0 }}
        >
          ← Back
        </button>
        <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>Tax Documents</h1>
      </div>

      <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem", marginBottom: "1.5rem" }}>
        <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "12px" }}>
          Upload a document
        </div>

        <div style={{ display: "flex", gap: "8px", marginBottom: "8px" }}>
          {(["w2", "1099", "1098"] as DocumentType[]).map(t => (
            <button
              key={t}
              onClick={() => setSelectedType(t)}
              style={{
                padding: "6px 14px", borderRadius: "6px", border: "none",
                background: selectedType === t ? "#1a1a1a" : "#e8e8e8",
                color: selectedType === t ? "white" : "#555",
                fontSize: "13px", fontWeight: 500, cursor: "pointer",
              }}
            >
              {docTypeLabels[t]}
            </button>
          ))}
        </div>

        <div style={{ fontSize: "12px", color: "#999", marginBottom: "12px" }}>
          {docTypeDescriptions[selectedType]}
        </div>

        <div style={{ display: "flex", gap: "10px", alignItems: "center" }}>
          <label style={{
            display: "inline-block", padding: "8px 14px",
            background: "white", border: "1px solid #e0e0e0", borderRadius: "8px",
            fontSize: "13px", color: file ? "#222" : "#888", cursor: "pointer",
            flexShrink: 0, maxWidth: "200px", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
          }}>
            {file ? file.name : "Choose PDF…"}
            <input
              ref={fileInputRef}
              type="file"
              accept="application/pdf,.pdf"
              style={{ display: "none" }}
              onChange={e => { setFile(e.target.files?.[0] || null); setUploadError(null); }}
            />
          </label>
          <button
            onClick={handleUpload}
            disabled={!file || uploading}
            style={{
              flex: 1, padding: "8px 16px",
              background: (!file || uploading) ? "#ccc" : "#1a1a1a",
              color: "white", border: "none", borderRadius: "8px",
              fontSize: "13px", fontWeight: 500,
              cursor: (!file || uploading) ? "not-allowed" : "pointer",
            }}
          >
            {uploading ? "Extracting…" : "Upload & extract"}
          </button>
        </div>

        {uploading && (
          <div style={{ color: "#888", fontSize: "12px", marginTop: "10px" }}>
            Claude is reading your document and extracting key fields…
          </div>
        )}

        {uploadError && (
          <div style={{ color: "#c0392b", fontSize: "12px", marginTop: "8px", padding: "8px 10px", background: "#fdf0ee", borderRadius: "6px" }}>
            {uploadError}
          </div>
        )}
      </div>

      <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "12px" }}>
        Uploaded documents
      </div>

      {loading && (
        <div style={{ color: "#888", fontSize: "14px", textAlign: "center", padding: "2rem 0" }}>Loading…</div>
      )}

      {!loading && docs.length === 0 && (
        <div style={{ color: "#999", fontSize: "14px", textAlign: "center", padding: "2rem 0", lineHeight: "1.6" }}>
          No documents yet.<br />
          Upload a W-2, 1099, or 1098 to help Claude factor in your real income and tax data when making investment recommendations.
        </div>
      )}

      {docs.map(doc => (
        <DocCard
          key={doc.ID}
          doc={doc}
          onDelete={() => setDocs(prev => prev.filter(d => d.ID !== doc.ID))}
        />
      ))}
    </div>
  );
}
