"use client";

import { LIFECYCLE_STEPS, STATE_DISPLAY, getStateName, type VaultStateName } from "@/lib/constants";

interface StateTimelineProps {
  currentStateIndex: number;
}

export function StateTimeline({ currentStateIndex }: StateTimelineProps) {
  const currentStateName = getStateName(currentStateIndex);

  // Find position of current state in the lifecycle steps
  const currentPos = LIFECYCLE_STEPS.indexOf(currentStateName);
  // If state isn't in the main lifecycle (e.g. CLOSED, SLASHING_REVIEW), show as "off track"
  const isTerminal = currentStateName === "CLOSED" || currentStateName === "FULLY_RELEASED";

  return (
    <div className="timeline">
      {LIFECYCLE_STEPS.map((step, idx) => {
        const display = STATE_DISPLAY[step];
        let status: "completed" | "current" | "future";

        if (isTerminal && currentStateName === step) {
          status = "current";
        } else if (currentPos >= 0) {
          if (idx < currentPos) status = "completed";
          else if (idx === currentPos) status = "current";
          else status = "future";
        } else {
          status = "future";
        }

        return (
          <div key={step} style={{ display: "flex", alignItems: "flex-start", flex: idx < LIFECYCLE_STEPS.length - 1 ? 1 : undefined }}>
            <div className={`timeline-step ${status}`}>
              <div className="timeline-dot" />
              <span className="timeline-label">{display.label}</span>
            </div>
            {idx < LIFECYCLE_STEPS.length - 1 && (
              <div
                className={`timeline-connector ${status === "completed" ? "completed" : ""}`}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
