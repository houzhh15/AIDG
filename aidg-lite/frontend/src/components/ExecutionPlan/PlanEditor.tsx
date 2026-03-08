import React from 'react';
import Editor from '@monaco-editor/react';

interface PlanEditorProps {
  value: string;
  onChange: (value: string | undefined) => void;
  height?: string;
}

const PlanEditor: React.FC<PlanEditorProps> = ({ 
  value, 
  onChange, 
  height = '600px' 
}) => {
  return (
    <Editor
      height={height}
      defaultLanguage="markdown"
      value={value}
      onChange={onChange}
      theme="vs-dark"
      options={{
        minimap: { enabled: false },
        fontSize: 14,
        lineNumbers: 'on',
        scrollBeyondLastLine: false,
        wordWrap: 'on',
        wrappingIndent: 'indent',
        folding: true,
        renderWhitespace: 'boundary',
        tabSize: 2
      }}
    />
  );
};

export default PlanEditor;
