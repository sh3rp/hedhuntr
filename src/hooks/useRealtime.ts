import { useEffect, useMemo, useRef, useState } from "react";
import type { RealtimeEvent } from "../types";

type ConnectionState = "connecting" | "connected" | "disconnected";

const topics = ["jobs", "applications", "notifications", "source_runs"];

export function useRealtime(onEvent?: (event: RealtimeEvent) => void) {
  const [state, setState] = useState<ConnectionState>("connecting");
  const [events, setEvents] = useState<RealtimeEvent[]>([]);
  const retryRef = useRef<number | undefined>(undefined);
  const attemptsRef = useRef(0);
  const onEventRef = useRef(onEvent);

  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  const wsUrl = useMemo(() => {
    const configured = import.meta.env.VITE_HEDHUNTR_WS_URL as string | undefined;
    if (configured) return configured;
    const apiUrl = import.meta.env.VITE_HEDHUNTR_API_URL as string | undefined;
    if (apiUrl) {
      const parsed = new URL(apiUrl, window.location.href);
      parsed.protocol = parsed.protocol === "https:" ? "wss:" : "ws:";
      parsed.pathname = "/ws";
      parsed.search = "";
      parsed.hash = "";
      return parsed.toString();
    }
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${window.location.host}/ws`;
  }, []);

  useEffect(() => {
    let closed = false;
    let socket: WebSocket | undefined;

    const scheduleReconnect = () => {
      if (closed) return;
      attemptsRef.current += 1;
      const delay = Math.min(30000, 1000 * 2 ** Math.min(attemptsRef.current, 5));
      retryRef.current = window.setTimeout(connect, delay);
    };

    const connect = () => {
      setState("connecting");
      try {
        socket = new WebSocket(wsUrl);
      } catch {
        setState("disconnected");
        scheduleReconnect();
        return;
      }

      socket.addEventListener("open", () => {
        attemptsRef.current = 0;
        setState("connected");
        socket?.send(JSON.stringify({ type: "subscribe", topics }));
      });

      socket.addEventListener("message", (message) => {
        try {
          const parsed = JSON.parse(message.data as string) as RealtimeEvent;
          setEvents((current) => [parsed, ...current].slice(0, 12));
          if (parsed.type === "event") {
            onEventRef.current?.(parsed);
          }
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
        scheduleReconnect();
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
