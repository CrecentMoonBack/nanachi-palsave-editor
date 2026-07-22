import React from "react";

/**
 * Catches a render error and shows it, instead of leaving a black window.
 *
 * React unmounts the whole tree when a render throws. Without this, one bad
 * field anywhere blanks the entire app with nothing on screen and nothing in
 * a log the user can reach — which is exactly how a nil slice serialising as
 * JSON null presented: click a pal with no passive skills, and the editor
 * disappears along with everything else.
 *
 * This does not make the bug harmless. It makes it reportable: the message and
 * the component stack are on screen, and the rest of the session is still
 * there behind a reload.
 */
export class ErrorBoundary extends React.Component<
  { children: React.ReactNode },
  { error: Error | null; stack: string }
> {
  constructor(props: { children: React.ReactNode }) {
    super(props);
    this.state = { error: null, stack: "" };
  }

  static getDerivedStateFromError(error: Error) {
    return { error, stack: "" };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    // Kept so a screenshot of this screen is enough to locate the fault.
    this.setState({ error, stack: info.componentStack ?? "" });
  }

  render() {
    const { error, stack } = this.state;
    if (!error) return this.props.children;

    return (
      <div className="crash">
        <h2>화면을 그리다가 문제가 생겼습니다</h2>
        <p>
          세이브 파일은 건드리지 않았습니다. 저장하지 않았다면 아무것도 바뀌지
          않습니다.
        </p>
        <pre>{String(error?.stack || error)}</pre>
        {stack && <pre className="dim">{stack}</pre>}
        <button className="primary" onClick={() => location.reload()}>
          다시 시작
        </button>
      </div>
    );
  }
}
