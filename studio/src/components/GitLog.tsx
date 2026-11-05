import React, { useEffect, useRef } from 'react';
import { GitEvent } from '../types';

interface GitLogProps {
  events: GitEvent[];
}

export const GitLog: React.FC<GitLogProps> = ({ events }) => {
  const preRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (preRef.current) {
      preRef.current.scrollTop = preRef.current.scrollHeight;
    }
  }, [events]);

  return (
    <div>
      <div className="pane-head">
        Git activity <span className="muted">live via /studio/api/events</span>
      </div>
      <pre ref={preRef} className="gitlog" data-testid="git-log">
        {events.length === 0 ? (
          <span className="note">  Waiting for git commands…{'\n'}</span>
        ) : (
          events.map((ev, idx) => {
            const cmd = `git ${ev.args.join(' ')}`;
            return (
              <React.Fragment key={`${ev.time}-${idx}`}>
                <span className="cmd">{`$ ${cmd}\n`}</span>
                {ev.error ? (
                  <span className="err">{`  ✗ ${ev.error}\n`}</span>
                ) : ev.output ? (
                  <span className="ok">{`  ${ev.output.replace(/\n/g, '\n  ')}\n`}</span>
                ) : null}
              </React.Fragment>
            );
          })
        )}
      </pre>
    </div>
  );
};
