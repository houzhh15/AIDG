import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import { PermissionProvider } from './contexts/PermissionContext';
import 'antd/dist/reset.css';
import './global.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <PermissionProvider>
    <App />
  </PermissionProvider>
);
