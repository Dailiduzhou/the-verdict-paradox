import {
  Alert,
  Box,
  Button,
  Link,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { useState } from 'react'

type Mode = 'login' | 'register'

type Props = {
  initialMode?: Mode
  onLogin: (username: string, password: string) => Promise<void>
  onRegister: (username: string, password: string) => Promise<void>
}

export default function AuthModal({ initialMode = 'login', onLogin, onRegister }: Props) {
  const [mode, setMode] = useState<Mode>(initialMode)

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async () => {
    setError(null)
    const u = username.trim()
    if (!u) return setError('请输入用户名')
    if (!password) return setError('请输入密码')

    setBusy(true)
    try {
      if (mode === 'login') {
        await onLogin(u, password)
      } else {
        if (!confirmPassword) return setError('请确认密码')
        if (password !== confirmPassword) return setError('两次密码不一致')
        await onRegister(u, password)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : '操作失败')
    } finally {
      setBusy(false)
    }
  }

  const flip = () => {
    setError(null)
    setMode((m) => (m === 'login' ? 'register' : 'login'))
  }

  return (
    <Box
      sx={{
        position: 'fixed',
        inset: 0,
        display: 'grid',
        placeItems: 'center',
        p: 2,
      }}
    >
      <Paper
        elevation={10}
        sx={{
          width: 'min(420px, 100%)',
          p: 3,
          backdropFilter: 'blur(8px)',
          backgroundImage:
            'linear-gradient(180deg, rgba(255,255,255,0.06), rgba(255,255,255,0.02))',
          border: '1px solid rgba(255,255,255,0.12)',
        }}
      >
        <Stack spacing={2}>
          <Typography variant="h6" sx={{ fontWeight: 600 }}>
            {mode === 'login' ? '登录' : '注册'}
          </Typography>

          {error ? <Alert severity="error">{error}</Alert> : null}

          <TextField
            autoFocus
            label="用户名"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            disabled={busy}
            fullWidth
          />
          <TextField
            label="密码"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={busy}
            fullWidth
          />
          {mode === 'register' ? (
            <TextField
              label="确认密码"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              disabled={busy}
              fullWidth
            />
          ) : null}

          <Button variant="contained" size="large" disabled={busy} onClick={submit}>
            {mode === 'login' ? '登录' : '注册'}
          </Button>

          <Box sx={{ display: 'flex', justifyContent: 'center' }}>
            <Link
              component="button"
              type="button"
              underline="always"
              color="text.secondary"
              disabled={busy}
              onClick={flip}
              sx={{ fontSize: 13 }}
            >
              {mode === 'login' ? '注册' : '登录'}
            </Link>
          </Box>
        </Stack>
      </Paper>
    </Box>
  )
}
