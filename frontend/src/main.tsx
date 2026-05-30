import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { CssBaseline, ThemeProvider, createTheme } from '@mui/material'

import App from './App'
import './index.css'

const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#90caf9' },
    background: { default: '#0b0c10', paper: '#11131a' },
  },
  shape: { borderRadius: 12 },
  typography: {
    fontFamily: [
      'system-ui',
      '-apple-system',
      'Segoe UI',
      'Roboto',
      'Noto Sans',
      'Helvetica',
      'Arial',
      'sans-serif',
    ].join(','),
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <App />
    </ThemeProvider>
  </StrictMode>,
)
