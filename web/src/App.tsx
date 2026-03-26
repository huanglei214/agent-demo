import { useEffect, useState, type MouseEvent } from "react";

import { ErrorBoundary } from "./components/ErrorBoundary";
import { JsonBlock } from "./components/JsonBlock";
import { ChatPage } from "./pages/ChatPage";
import { RunDetailsPage } from "./pages/RunDetailsPage";
import { RunLauncherPage } from "./pages/RunLauncherPage";
import { SessionDetailsPage } from "./pages/SessionDetailsPage";
import { getHealth } from "./lib/api";
import { useI18n } from "./lib/i18n";

type Route =
  | { name: "launchpad" }
  | { name: "chat" }
  | { name: "run"; runId: string }
  | { name: "session"; sessionId: string };

function parseRoute(pathname: string): Route {
  const segments = pathname.split("/").filter(Boolean);
  if (segments[0] === "runs" && segments[1]) {
    return { name: "run", runId: segments[1] };
  }
  if (segments[0] === "launchpad") {
    return { name: "launchpad" };
  }
  if (segments[0] === "chat") {
    return { name: "chat" };
  }
  if (segments[0] === "sessions" && segments[1]) {
    return { name: "session", sessionId: segments[1] };
  }
  return { name: "chat" };
}

function navigate(pathname: string) {
  window.history.pushState({}, "", pathname);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

export default function App() {
  const { language, setLanguage, copy } = useI18n();
  const [route, setRoute] = useState<Route>(() => parseRoute(window.location.pathname));
  const [health, setHealth] = useState<string>("checking");

  useEffect(() => {
    const onPopState = () => setRoute(parseRoute(window.location.pathname));
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

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
      {route.name === "chat" ? (
        <ChatPage
          workspace=""
          healthLabel={formatHealthLabel(health, copy)}
          healthState={health}
          language={language}
          onLanguageChange={setLanguage}
          onOpenSession={(sessionId) => navigate(`/sessions/${sessionId}`)}
          onOpenRun={(runId) => navigate(`/runs/${runId}`)}
        />
      ) : null}
      {route.name !== "chat" ? (
      <div className="app-shell">
        <header className="hero">
          <div>
            <p className="eyebrow">{copy.app.eyebrow}</p>
            <h1>{copy.app.title}</h1>
            <p className="lede">{copy.app.lede}</p>
          </div>
          <div className="hero-actions">
            <div className="language-switch" aria-label={copy.app.language.label}>
              <span className="language-label">{copy.app.language.label}</span>
              <button
                className={language === "en" ? "language-button active" : "language-button"}
                type="button"
                onClick={() => setLanguage("en")}
              >
                {copy.app.language.english}
              </button>
              <button
                className={language === "zh" ? "language-button active" : "language-button"}
                type="button"
                onClick={() => setLanguage("zh")}
              >
                {copy.app.language.chinese}
              </button>
            </div>
            <div className={`health health-${health.replace(/\s+/g, "-")}`}>
              <span className="health-dot" />
              <span>{formatHealthLabel(health, copy)}</span>
            </div>
          </div>
        </header>

        <nav className="top-nav">
          <a href="/launchpad" onClick={(event) => linkTo(event, "/launchpad")}>
            {copy.app.navLaunchpad}
          </a>
          <a href="/" onClick={(event) => linkTo(event, "/")}>
            {copy.app.navChat}
          </a>
          <a href="/sessions/demo" onClick={(event) => linkTo(event, "/sessions/demo")}>
            {copy.app.navSession}
          </a>
          <a href="/runs/demo" onClick={(event) => linkTo(event, "/runs/demo")}>
            {copy.app.navRun}
          </a>
        </nav>

        <main className="main-grid">
          <section className="main-panel">
            {route.name === "launchpad" ? (
              <RunLauncherPage
                onOpenRun={(runId) => navigate(`/runs/${runId}`)}
                onOpenSession={(sessionId) => navigate(`/sessions/${sessionId}`)}
              />
            ) : null}
            {route.name === "session" ? (
              <SessionDetailsPage
                sessionId={route.sessionId}
                onOpenRun={(runId) => navigate(`/runs/${runId}`)}
              />
            ) : null}
            {route.name === "run" ? (
              <RunDetailsPage
                runId={route.runId}
                onOpenSession={(sessionId) => navigate(`/sessions/${sessionId}`)}
              />
            ) : null}
          </section>

          <aside className="side-panel">
            <div className="panel-card">
              <h2>{copy.app.routingNotesTitle}</h2>
              <p>{copy.app.routingNotesBody}</p>
            </div>
            <div className="panel-card">
              <h2>{copy.app.usefulPathsTitle}</h2>
              <JsonBlock
                value={{
                  home: copy.app.paths.home,
                  chat: copy.app.paths.chat,
                  session: copy.app.paths.session,
                  run: copy.app.paths.run,
                  api: copy.app.paths.api,
                }}
              />
            </div>
          </aside>
        </main>
      </div>
      ) : null}
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

function linkTo(event: MouseEvent<HTMLAnchorElement>, pathname: string) {
  event.preventDefault();
  navigate(pathname);
}
