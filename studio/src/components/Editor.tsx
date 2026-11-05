import React, { useEffect, useImperativeHandle, useRef, useState } from 'react';
import { EditorState } from '@codemirror/state';
import { EditorView, keymap, lineNumbers, drawSelection } from '@codemirror/view';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { markdown } from '@codemirror/lang-markdown';
import { json } from '@codemirror/lang-json';
import { LanguageDescription } from '@codemirror/language';
import { BLOCK_NAMES, BLOCK_TEMPLATES, CustomBlockName } from '../utils';

export interface EditorHandle {
  jumpToLine: (line: number) => void;
  insertSnippet: (snippet: string) => void;
  focus: () => void;
}

interface EditorProps {
  value: string;
  filePath: string;
  disabled?: boolean;
  onChange: (next: string) => void;
  onUploadFile: (file: File, fromDrop: boolean) => void;
}

const codeLanguages = [
  ...BLOCK_NAMES.map((name) =>
    LanguageDescription.of({
      name,
      support: json(),
    }),
  ),
  LanguageDescription.of({
    name: 'json',
    support: json(),
  }),
];

export const Editor = React.forwardRef<EditorHandle, EditorProps>(
  ({ value, filePath, disabled, onChange, onUploadFile }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const viewRef = useRef<EditorView | null>(null);
    const onChangeRef = useRef(onChange);
    onChangeRef.current = onChange;
    const [dragOver, setDragOver] = useState(false);

    useEffect(() => {
      if (!containerRef.current) return;

      const state = EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          history(),
          drawSelection(),
          EditorView.lineWrapping,
          keymap.of([indentWithTab, ...defaultKeymap, ...historyKeymap]),
          markdown({ codeLanguages }),
          EditorView.contentAttributes.of({ 'aria-label': 'Markdown body' }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              onChangeRef.current(update.state.doc.toString());
            }
          }),
        ],
      });

      const view = new EditorView({
        state,
        parent: containerRef.current,
      });
      viewRef.current = view;

      return () => {
        view.destroy();
        viewRef.current = null;
      };
      // Initialize once; subsequent external value changes are synced below.
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    useEffect(() => {
      const view = viewRef.current;
      if (!view) return;
      const current = view.state.doc.toString();
      if (current !== value) {
        view.dispatch({
          changes: { from: 0, to: current.length, insert: value },
        });
      }
    }, [value]);

    const insertSnippet = (snippet: string) => {
      const view = viewRef.current;
      if (!view) {
        onChangeRef.current(value + snippet);
        return;
      }
      const { from, to } = view.state.selection.main;
      view.dispatch({
        changes: { from, to, insert: snippet },
        selection: { anchor: from + snippet.length },
        scrollIntoView: true,
      });
      view.focus();
    };

    const jumpToLine = (lineNum: number) => {
      const view = viewRef.current;
      if (!view) return;
      const clamped = Math.max(1, Math.min(lineNum, view.state.doc.lines));
      const line = view.state.doc.line(clamped);
      view.dispatch({
        selection: { anchor: line.from, head: line.to },
        scrollIntoView: true,
      });
      view.focus();
    };

    useImperativeHandle(ref, () => ({
      jumpToLine,
      insertSnippet,
      focus: () => viewRef.current?.focus(),
    }));

    const handleInsertBlock = (blockName: CustomBlockName) => {
      const snippet = `\n\`\`\`${blockName}\n${BLOCK_TEMPLATES[blockName]}\n\`\`\`\n`;
      insertSnippet(snippet);
    };

    const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
      e.preventDefault();
      setDragOver(false);
      if (disabled) return;
      const file = e.dataTransfer.files?.[0];
      if (file) {
        onUploadFile(file, true);
      }
    };

    const handleFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) {
        onUploadFile(file, true);
      }
      e.target.value = '';
    };

    return (
      <div className="pane">
        <div className="pane-head">
          Markdown <span className="muted">{filePath}</span>
          <span className="insert" aria-label="Insert block">
            {BLOCK_NAMES.map((b) => (
              <button
                key={b}
                type="button"
                disabled={disabled}
                onClick={() => handleInsertBlock(b)}
              >
                {b}
              </button>
            ))}
            <label className="upload-btn" title="Upload asset (.svg, .png, .jpg, .webp, .gif) or demo (.html)">
              + asset
              <input
                type="file"
                accept=".svg,.png,.jpg,.jpeg,.webp,.gif,.html"
                style={{ display: 'none' }}
                disabled={disabled}
                onChange={handleFileInput}
              />
            </label>
          </span>
        </div>
        <div
          ref={containerRef}
          data-testid="editor-dropzone"
          className={`editor-wrap${dragOver ? ' drag-over' : ''}`}
          onDragOver={(e) => {
            e.preventDefault();
            if (!disabled) setDragOver(true);
          }}
          onDragLeave={() => setDragOver(false)}
          onDrop={handleDrop}
        />
      </div>
    );
  },
);

Editor.displayName = 'Editor';
