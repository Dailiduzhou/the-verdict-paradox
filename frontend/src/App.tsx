import { useEffect, useRef, useState } from 'react'
import splashSvg from './assets/login.svg'
import loginSvg from './assets/login1.svg'
import registerSvg from './assets/register.svg'

type View = 'splash' | 'login' | 'register' | 'game'

const TOKEN_KEY = 'token'

async function login(username: string, password: string) {
  const res = await fetch('/v1/users/login', {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
    },
    // Backend contract is { phone, password }. Treat username as phone here.
    body: JSON.stringify({ phone: username, password }),
  })

  if (!res.ok) throw new Error('login_failed')
  const data = (await res.json()) as { token?: string }
  if (!data.token) throw new Error('missing_token')
  return data.token
}

async function register(username: string, password: string) {
  const res = await fetch('/v1/users/register', {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
    },
    body: JSON.stringify({ username, password }),
  })

  if (!res.ok) throw new Error('register_failed')
}

async function verifyToken(token: string) {
  try {
    const res = await fetch('/v1/users/verify', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
      },
      body: JSON.stringify({ token }),
    })

    if (!res.ok) return false
    const data = (await res.json()) as { valid?: boolean }
    return data.valid === true
  } catch {
    return false
  }
}

function Game() {
  return (
    <section className="game" aria-label="game">
      <div className="gameInner">GAME</div>
    </section>
  )
}

function Toast(props: { message: string }) {
  return (
    <div className="toast" role="status" aria-live="polite">
      {props.message}
    </div>
  )
}

function Register(props: { onRegistered: () => void; toast: (message: string) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [password2, setPassword2] = useState('')
  const [pending, setPending] = useState(false)

  const submit = async () => {
    if (pending) return
    if (!username.trim() || !password) {
      props.toast('请填写用户名和密码')
      return
    }
    if (password !== password2) {
      props.toast('两次密码不一致')
      return
    }

    setPending(true)
    try {
      await register(username.trim(), password)
      props.onRegistered()
    } catch {
      props.toast('注册失败')
    } finally {
      setPending(false)
    }
  }

  return (
    <main className="page" aria-label="register">
      <div className="frame">
        <div className="canvas">
          <img className="bg" src={registerSvg} alt="" aria-hidden="true" />

          <div className="registerOverlay">
            <input
              className="loginField registerFieldUser"
              type="text"
              placeholder="用户名"
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <input
              className="loginField registerFieldPass"
              type="password"
              placeholder="密码"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <input
              className="loginField registerFieldPass2"
              type="password"
              placeholder="确认密码"
              autoComplete="new-password"
              value={password2}
              onChange={(e) => setPassword2(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') submit()
              }}
            />

            <button
              className="registerSubmit"
              type="button"
              onClick={submit}
              disabled={pending}
            >
              注册
            </button>
          </div>
        </div>
      </div>
    </main>
  )
}

function Login(props: { onLoggedIn: () => void; onRegister: () => void; toast: (message: string) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [pending, setPending] = useState(false)

  const submit = async () => {
    if (pending) return
    setPending(true)
    try {
      const token = await login(username.trim(), password)
      localStorage.setItem(TOKEN_KEY, token)
      props.onLoggedIn()
    } catch {
      props.toast('登录失败')
    } finally {
      setPending(false)
    }
  }

  return (
    <main className="page" aria-label="login">
      <div className="frame">
        <div className="canvas">
          <img className="bg" src={loginSvg} alt="" aria-hidden="true" />

          <input
            className="loginField loginFieldUser"
            type="text"
            placeholder="用户名"
            autoComplete="username"
            inputMode="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
          <input
            className="loginField loginFieldPass"
            type="password"
            placeholder="密码"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') submit()
            }}
          />

          <button className="loginBtn" type="button" onClick={submit} disabled={pending}>
            登录
          </button>

          <button className="registerBtn" type="button" onClick={props.onRegister} aria-label="register" />
        </div>
      </div>
    </main>
  )
}

export default function App() {
  const [view, setView] = useState<View>('splash')
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const toastTimerRef = useRef<number | null>(null)

  const toast = (message: string) => {
    setToastMessage(message)
    if (toastTimerRef.current != null) window.clearTimeout(toastTimerRef.current)
    toastTimerRef.current = window.setTimeout(() => setToastMessage(null), 2200)
  }

  useEffect(() => {
    return () => {
      if (toastTimerRef.current != null) window.clearTimeout(toastTimerRef.current)
    }
  }, [])

  useEffect(() => {
    const onKeyDown = async () => {
      if (view !== 'splash') return

      const token = localStorage.getItem(TOKEN_KEY)
      if (!token) {
        setView('login')
        return
      }

      const valid = await verifyToken(token)
      if (valid) {
        setView('game')
      } else {
        localStorage.removeItem(TOKEN_KEY)
        setView('login')
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [view])

  if (view === 'game') return <Game />

  if (view === 'login') {
    return (
      <>
        <Login
          onLoggedIn={() => setView('game')}
          onRegister={() => setView('register')}
          toast={toast}
        />
        {toastMessage && <Toast message={toastMessage} />}
      </>
    )
  }

  if (view === 'register') {
    return (
      <>
        <Register onRegistered={() => setView('login')} toast={toast} />
        {toastMessage && <Toast message={toastMessage} />}
      </>
    )
  }

  return (
    <>
      <main className="page">
        <img
          className="art"
          src={view === 'splash' ? splashSvg : loginSvg}
          alt=""
          aria-hidden="true"
        />
      </main>
      {toastMessage && <Toast message={toastMessage} />}
    </>
  )
}
