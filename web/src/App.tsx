import { useEffect, useState } from "react";

import { ErrorBoundary } from "./components/ErrorBoundary";
import { ChatPage } from "./pages/ChatPage";
import { getHealth } from "./lib/api";
import { useI18n } from "./lib/i18n";

export default function App() {
  const { language, setLanguage, copy } = useI18n();
  const [health, setHealth] = useState<string>("checking");

  useEffect(() => {
    let active = true;
    getHealth()
      .then(() => {
        if (active) {
          setHealth("connected");
        }
      })
      .catch(() => {
        if (active) {
          setHealth("server offline");
        }
      });
    return () => {
      active = false;
    };
  }, []);

  return (
    <ErrorBoundary
      eyebrow={copy.errorBoundary.eyebrow}
      title={copy.errorBoundary.title}
      body={copy.errorBoundary.body}
      backLabel={copy.errorBoundary.back}
      reloadLabel={copy.errorBoundary.reload}
      fallbackMessage={copy.errorBoundary.unexpected}
    >
      <ChatPage
        workspace=""
        healthLabel={formatHealthLabel(health, copy)}
        healthState={health}
        language={language}
        onLanguageChange={setLanguage}
      />
    </ErrorBoundary>
  );
}

function formatHealthLabel(health: string, copy: ReturnType<typeof useI18n>["copy"]) {
  if (health === "connected") {
    return copy.app.health.connected;
  }
  if (health === "server offline") {
    return copy.app.health.serverOffline;
  }
  return copy.app.health.checking;
}
