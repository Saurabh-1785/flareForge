"use client";

import { useState, useEffect } from "react";

interface CountdownTimerProps {
  deadline: number; // Unix timestamp in seconds
  label?: string;
}

export function CountdownTimer({ deadline, label }: CountdownTimerProps) {
  const [remaining, setRemaining] = useState<number>(0);

  useEffect(() => {
    const update = () => {
      const now = Math.floor(Date.now() / 1000);
      setRemaining(Math.max(0, deadline - now));
    };

    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
  }, [deadline]);

  const isExpired = remaining === 0 && deadline > 0;
  const isUrgent = remaining > 0 && remaining < 60;

  const hours = Math.floor(remaining / 3600);
  const minutes = Math.floor((remaining % 3600) / 60);
  const seconds = remaining % 60;

  const pad = (n: number) => n.toString().padStart(2, "0");

  const statusClass = isExpired
    ? "countdown-expired"
    : isUrgent
      ? "countdown-urgent"
      : "countdown-ok";

  return (
    <div className="stat">
      {label && <span className="stat-label">{label}</span>}
      <span className={`countdown ${statusClass}`}>
        {deadline === 0 ? (
          "—"
        ) : isExpired ? (
          "EXPIRED"
        ) : (
          <>
            {hours > 0 && <>{pad(hours)}:</>}
            {pad(minutes)}:{pad(seconds)}
          </>
        )}
      </span>
      {!isExpired && deadline > 0 && (
        <span className="countdown-label">
          {new Date(deadline * 1000).toLocaleTimeString()}
        </span>
      )}
    </div>
  );
}
