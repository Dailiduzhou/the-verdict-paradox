import { Box, CircularProgress, Typography } from '@mui/material'
import { useEffect, useRef, useState } from 'react'

import { postJSON } from './api'
import AuthModal from './auth/AuthModal'
import Game from './Game'

type Screen = 'splash' | 'checking' | 'auth' | 'game'

function getToken() {
  return localStorage.getItem('token') || ''
}

function setToken(token: string) {
  localStorage.setItem('token', token)
}

function clearToken() {
  localStorage.removeItem('token')
}

export default function App() {
  const [screen, setScreen] = useState<Screen>('splash')
  const checkingRef = useRef(false)

  const enter = async () => {
    if (checkingRef.current) return
    checkingRef.current = true
    setScreen('checking')

    const token = getToken()
    if (!token) {
      checkingRef.current = false
      setScreen('auth')
      return
    }

    try {
      const res = await postJSON<{ valid: boolean }>('/token/verify', { token })
      if (res?.valid) {
        setScreen('game')
      } else {
        clearToken()
        setScreen('auth')
      }
    } catch {
      clearToken()
      setScreen('auth')
    } finally {
      checkingRef.current = false
    }
  }

  useEffect(() => {
    if (screen !== 'splash') return

    const onEnter = () => void enter()
    window.addEventListener('keydown', onEnter, { once: true })
    window.addEventListener('pointerdown', onEnter, { once: true })
    return () => {
      window.removeEventListener('keydown', onEnter)
      window.removeEventListener('pointerdown', onEnter)
    }
  }, [screen])

  const onLogin = async (username: string, password: string) => {
    const res = await postJSON<{ token?: string; msg?: string }>(
      '/auth/login',
      { username, password },
    )

    if (!res?.token) throw new Error(res?.msg || '登录失败')
    setToken(res.token)
    setScreen('game')
  }

  const onRegister = async (username: string, password: string) => {
    const res = await postJSON<{ token?: string; msg?: string }>(
      '/auth/register',
      { username, password },
    )

    if (!res?.token) throw new Error(res?.msg || '注册失败')
    setToken(res.token)
    setScreen('game')
  }

  return (
    <Box sx={{ height: '100svh', width: '100vw', overflow: 'hidden' }}>
      <Box
        sx={{
          position: 'fixed',
          inset: 0,
          background:
            'radial-gradient(900px 500px at 30% 30%, rgba(90,170,255,0.18), transparent 60%),\nradial-gradient(700px 500px at 70% 70%, rgba(170,90,255,0.14), transparent 60%),\nlinear-gradient(180deg, #07080c, #0b0c10 60%, #07080c)',
          transform: 'scale(1.02)',
        }}
      />
      <Box
        sx={{
          position: 'fixed',
          inset: 0,
          background:
            'radial-gradient(1200px 700px at 50% 30%, rgba(0,0,0,0), rgba(0,0,0,0.55) 70%, rgba(0,0,0,0.75))',
          pointerEvents: 'none',
        }}
      />

      {screen === 'splash' ? (
        <Box
          sx={{
            position: 'fixed',
            inset: 0,
            display: 'grid',
            placeItems: 'center',
            p: 2,
          }}
        >
          <Typography
            variant="h4"
            className="blink"
            sx={{
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
              textAlign: 'center',
              userSelect: 'none',
            }}
          >
            按任意键进入游戏
          </Typography>
        </Box>
      ) : null}

      {screen === 'checking' ? (
        <Box
          sx={{
            position: 'fixed',
            inset: 0,
            display: 'grid',
            placeItems: 'center',
          }}
        >
          <CircularProgress />
        </Box>
      ) : null}

      {screen === 'auth' ? (
        <AuthModal onLogin={onLogin} onRegister={onRegister} />
      ) : null}

      {screen === 'game' ? <Game /> : null}
    </Box>
  )
}
