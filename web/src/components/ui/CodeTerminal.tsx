type CodeTerminalProps = {
  lines: string[];
  prompt?: string;
};

export function CodeTerminal({ lines, prompt = "$" }: CodeTerminalProps) {
  return (
    <pre className="code-terminal">
      <span className="code-terminal__lights" aria-hidden="true">
        <i />
        <i />
        <i />
      </span>
      {lines.map((line) => (
        <code key={line}>
          <span>{prompt}</span> {line}
        </code>
      ))}
    </pre>
  );
}
