import { Component, type ErrorInfo, type ReactNode } from "react";

type ErrorBoundaryProps = {
  children: ReactNode;
  eyebrow: string;
  title: string;
  body: string;
  backLabel: string;
  reloadLabel: string;
  fallbackMessage: string;
};

type ErrorBoundaryState = {
  hasError: boolean;
  message: string;
};

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = {
    hasError: false,
    message: "",
  };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return {
      hasError: true,
      message: error.message || "",
    };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Harness UI crashed", error, info);
  }

  handleReset = () => {
    this.setState({
      hasError: false,
      message: "",
    });
    window.history.pushState({}, "", "/");
    window.dispatchEvent(new PopStateEvent("popstate"));
  };

  render() {
    if (!this.state.hasError) {
      return this.props.children;
    }

    return (
      <div className="error-shell">
        <div className="error-card">
          <p className="eyebrow">{this.props.eyebrow}</p>
          <h1>{this.props.title}</h1>
          <p className="lede">{this.props.body}</p>
          <pre className="json-block">{this.state.message || this.props.fallbackMessage}</pre>
          <div className="button-row">
            <button className="primary-button" type="button" onClick={this.handleReset}>
              {this.props.backLabel}
            </button>
            <button className="secondary-button" type="button" onClick={() => window.location.reload()}>
              {this.props.reloadLabel}
            </button>
          </div>
        </div>
      </div>
    );
  }
}
