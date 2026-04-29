import { useEffect, useMemo, useRef, useState } from "react";
import type { RealtimeEvent } from "../types";

type ConnectionState = "connecting" | "connected" | "disconnected";

const topics = ["jobs", "applications", "notifications", "source_runs"];

export function useRealtime() {
  const [state, setState] = useState<ConnectionState>("connecting");
  const [events, setEvents] = useState<RealtimeEvent[]>([]);
  const retryRef = useRef<number | undefined>(undefined);

  const wsUrl = useMemo(() => {
    const configured = import.meta.env.VITE_HEDHUNTR_WS_URL as string | undefined;
    if (configured) return configured;
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${window.location.host}/ws`;
  }, []);

  useEffect(() => {
    let closed = false;
    let socket: WebSocket | undefined;

    const connect = () => {
      setState("connecting");
      try {
        socket = new WebSocket(wsUrl);
      } catch {
        setState("disconnected");
        retryRef.current = window.setTimeout(connect, 2500);
        return;
      }

      socket.addEventListener("open", () => {
        setState("connected");
        socket?.send(JSON.stringify({ type: "subscribe", topics }));
      });

      socket.addEventListener("message", (message) => {
        try {
          const parsed = JSON.parse(message.data as string) as RealtimeEvent;
          setEvents((current) => [parsed, ...current].slice(0, 12));
        } catch {
          setEvents((current) => [
            { type: "status" as const, topic: "system", event_type: "InvalidMessage" },
            ...current
          ].slice(0, 12));
        }
      });

      socket.addEventListener("close", () => {
        if (closed) return;
        setState("disconnected");
        retryRef.current = window.setTimeout(connect, 2500);
      });

      socket.addEventListener("error", () => {
        setState("disconnected");
      });
    };

    connect();
    return () => {
      closed = true;
      if (retryRef.current) window.clearTimeout(retryRef.current);
      socket?.close();
    };
  }, [wsUrl]);

  return { state, events, wsUrl };
}
