import { useEffect, useRef, useState } from 'react'
import splashSvg from './assets/login.svg'
import loginSvg from './assets/login1.svg'
import registerSvg from './assets/register.svg'
import gameBgSvg from './assets/background.svg'

type View = 'splash' | 'login' | 'register' | 'game'

const TOKEN_KEY = 'token'
const USERNAME_KEY = 'username'

async function login(username: string, password: string) {
  const res = await fetch('/v1/users/login', {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
    },
    // Backend contract is { phone, password }. Treat username as phone here.
    body: JSON.stringify({ username, password }),
  })

  if (!res.ok) throw new Error('login_failed')
  const data = (await res.json()) as { token?: string }
  if (!data.token) throw new Error('missing_token')
  return data.token
}

async function startGame(name: string, token: string) {
  const res = await fetch('/v1/game/start', {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ name }),
  })

  if (!res.ok) throw new Error('start_failed')
  return (await res.json()) as { matchID?: string }
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

function Game(props: { toast: (message: string) => void; blocked: boolean }) {
  const [pressed, setPressed] = useState(false)
  const [matching, setMatching] = useState(false)
  const [dots, setDots] = useState(1)
  const dotsTimerRef = useRef<number | null>(null)

  useEffect(() => {
    if (!matching) return
    dotsTimerRef.current = window.setInterval(() => {
      setDots((d) => (d % 3) + 1)
    }, 450)
    return () => {
      if (dotsTimerRef.current != null) window.clearInterval(dotsTimerRef.current)
      dotsTimerRef.current = null
    }
  }, [matching])

  const onPressStart = () => {
    if (props.blocked || matching) return
    setPressed(true)
  }

  const onPressEnd = () => {
    setPressed(false)
  }

  const submit = async () => {
    if (props.blocked || matching) return

    const token = localStorage.getItem(TOKEN_KEY)
    const name = localStorage.getItem(USERNAME_KEY)

    if (!token) {
      props.toast('未登录')
      return
    }
    if (!name) {
      props.toast('缺少用户名')
      return
    }

    setMatching(true)
    setDots(1)
    try {
      await startGame(name, token)
    } catch {
      setMatching(false)
      props.toast('匹配失败')
    }
  }

  return (
    <section className="game" aria-label="game">
      <img className="gameBg" src={gameBgSvg} alt="" aria-hidden="true" />
      <div className="gameBottom" aria-hidden="true" />
      <button
        className={pressed ? 'enterBtn enterBtn--pressed' : 'enterBtn'}
        type="button"
        onPointerDown={onPressStart}
        onPointerUp={onPressEnd}
        onPointerCancel={onPressEnd}
        onPointerLeave={onPressEnd}
        onClick={submit}
        disabled={props.blocked}
      >
        <span className="enterBtnInner">
          {matching ? `Matching${'.'.repeat(dots)}` : 'Enter'}
        </span>
      </button>
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

function Fade(props: { on: boolean }) {
  return <div className={props.on ? 'fade fade--on' : 'fade'} aria-hidden="true" />
}

function Register(props: {
  onRegistered: () => void
  onBack: () => void
  toast: (message: string) => void
  blocked: boolean
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [password2, setPassword2] = useState('')
  const [pending, setPending] = useState(false)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      if (pending || props.blocked) return
      props.onBack()
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [pending, props.blocked, props.onBack])

  const submit = async () => {
    if (pending || props.blocked) return
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
              disabled={pending || props.blocked}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <input
              className="loginField registerFieldPass"
              type="password"
              placeholder="密码"
              autoComplete="new-password"
              disabled={pending || props.blocked}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <input
              className="loginField registerFieldPass2"
              type="password"
              placeholder="确认密码"
              autoComplete="new-password"
              disabled={pending || props.blocked}
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
              disabled={pending || props.blocked}
            >
              注册
            </button>
          </div>
        </div>
      </div>
    </main>
  )
}

function Login(props: {
  onLoggedIn: () => void
  onRegister: () => void
  onBack: () => void
  toast: (message: string) => void
  blocked: boolean
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [pending, setPending] = useState(false)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      if (pending || props.blocked) return
      props.onBack()
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [pending, props.blocked, props.onBack])

  const submit = async () => {
    if (pending || props.blocked) return
    setPending(true)
    try {
      const token = await login(username.trim(), password)
      localStorage.setItem(TOKEN_KEY, token)
      localStorage.setItem(USERNAME_KEY, username.trim())
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
            disabled={pending || props.blocked}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
          <input
            className="loginField loginFieldPass"
            type="password"
            placeholder="密码"
            autoComplete="current-password"
            disabled={pending || props.blocked}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') submit()
            }}
          />

          <button className="loginBtn" type="button" onClick={submit} disabled={pending || props.blocked}>
            登录
          </button>

          <button
            className="registerBtn"
            type="button"
            onClick={props.onRegister}
            disabled={pending || props.blocked}
            aria-label="register"
          />
        </div>
      </div>
    </main>
  )
}

export default function App() {
  const [view, setView] = useState<View>('splash')
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const toastTimerRef = useRef<number | null>(null)

  const [transitionOn, setTransitionOn] = useState(false)
  const [transitionBusy, setTransitionBusy] = useState(false)
  const transitionTimerRef = useRef<number | null>(null)
  const transitionBusyRef = useRef(false)

  const toast = (message: string) => {
    setToastMessage(message)
    if (toastTimerRef.current != null) window.clearTimeout(toastTimerRef.current)
    toastTimerRef.current = window.setTimeout(() => setToastMessage(null), 2200)
  }

  useEffect(() => {
    return () => {
      if (toastTimerRef.current != null) window.clearTimeout(toastTimerRef.current)
      if (transitionTimerRef.current != null) window.clearTimeout(transitionTimerRef.current)
    }
  }, [])

  const transitionTo = (next: View) => {
    if (transitionBusyRef.current) return
    if (next === view) return

    transitionBusyRef.current = true
    setTransitionBusy(true)
    setTransitionOn(true)

    // Fade to black.
    transitionTimerRef.current = window.setTimeout(() => {
      setView(next)

      // Next frame: start fading back in.
      requestAnimationFrame(() => {
        setTransitionOn(false)
        transitionTimerRef.current = window.setTimeout(() => {
          transitionBusyRef.current = false
          setTransitionBusy(false)
        }, 500)
      })
    }, 500)
  }

  useEffect(() => {
    const start = async () => {
      if (view !== 'splash') return
      if (transitionBusyRef.current) return

      const token = localStorage.getItem(TOKEN_KEY)
      if (!token) {
        transitionTo('login')
        return
      }

      const valid = await verifyToken(token)
      if (valid) {
        transitionTo('game')
      } else {
        localStorage.removeItem(TOKEN_KEY)
        transitionTo('login')
      }
    }

    const onKeyDown = () => {
      void start()
    }

    const onPointerDown = () => {
      void start()
    }

    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('pointerdown', onPointerDown, { passive: true })
    // Fallback for environments without Pointer Events.
    window.addEventListener('mousedown', onPointerDown, { passive: true })
    window.addEventListener('touchstart', onPointerDown, { passive: true })
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('mousedown', onPointerDown)
      window.removeEventListener('touchstart', onPointerDown)
    }
  }, [view])

  const blocked = transitionBusy

  if (view === 'game') {
    return (
      <>
        <Game toast={toast} blocked={blocked} />
        <Fade on={transitionOn} />
        {toastMessage && <Toast message={toastMessage} />}
      </>
    )
  }

  if (view === 'login') {
    return (
      <>
        <Login
          onLoggedIn={() => transitionTo('game')}
          onRegister={() => transitionTo('register')}
          onBack={() => transitionTo('splash')}
          toast={toast}
          blocked={blocked}
        />
        <Fade on={transitionOn} />
        {toastMessage && <Toast message={toastMessage} />}
      </>
    )
  }

  if (view === 'register') {
    return (
      <>
        <Register
          onRegistered={() => transitionTo('login')}
          onBack={() => transitionTo('login')}
          toast={toast}
          blocked={blocked}
        />
        <Fade on={transitionOn} />
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
      <Fade on={transitionOn} />
      {toastMessage && <Toast message={toastMessage} />}
    </>
  )
}
