import { useEffect, useRef, useState } from 'react'
import splashSvg from './assets/login.svg'
import loginSvg from './assets/login1.svg'
import registerSvg from './assets/register.svg'
import gameBgSvg from './assets/background.svg'
import tutorialSvg from './assets/tutorial.svg'
import settingsSvg from './assets/settings.svg'
import humanSvg from './assets/human.svg'
import spySvg from './assets/spy.svg'
import startrailSvg from './assets/startrail.svg'
import speakSvg from './assets/speak.svg'
import verdictSvg from './assets/verdict.svg'
import humanWinSvg from './assets/humanWin.svg'
import spyWinSvg from './assets/spyWin.svg'
import aiWinSvg from './assets/aiWin.svg'

type View = 'splash' | 'login' | 'register' | 'game'

const TOKEN_KEY = 'token'
const USERNAME_KEY = 'username'
const USER_ID_KEY = 'user_id'
const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '').trim().replace(/\/+$/, '')
const WS_BASE_URL = (import.meta.env.VITE_WS_BASE_URL ?? '').trim().replace(/\/+$/, '')

function apiUrl(path: string) {
  return API_BASE_URL ? `${API_BASE_URL}${path}` : path
}

function wsBaseUrl() {
  if (WS_BASE_URL) return WS_BASE_URL
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}`
}

async function login(username: string, password: string) {
  const res = await fetch(apiUrl('/v1/users/login'), {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
    },
    // Backend contract is { phone, password }. Treat username as phone here.
    body: JSON.stringify({ name: username, password }),
  })

  if (!res.ok) throw new Error('login_failed')
  const data = (await res.json()) as { id?: number; token?: string }
  if (!data.token) throw new Error('missing_token')
  return { token: data.token, id: data.id ?? 0 }
}

async function startGame(name: string, token: string) {
  const res = await fetch(apiUrl('/v1/game/start'), {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ name }),
  })

  if (!res.ok) throw new Error('start_failed')
  const data = (await res.json()) as { matchID?: string }
  if (!data.matchID) throw new Error('missing_match_id')
  return data.matchID
}

async function getMatchStatus(matchID: string, token: string) {
  const res = await fetch(apiUrl(`/v1/game/status/${encodeURIComponent(matchID)}`), {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  if (!res.ok) throw new Error('status_failed')
  return (await res.json()) as { status?: string; roomID?: string }
}

function toWsUrl(roomID: string, token: string) {
  const qs = new URLSearchParams({ token })
  return `${wsBaseUrl()}/ws/room/${encodeURIComponent(roomID)}?${qs.toString()}`
}

async function register(username: string, password: string) {
  const res = await fetch(apiUrl('/v1/users/register'), {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
    },
    body: JSON.stringify({ name: username, password }),
  })

  if (!res.ok) throw new Error('register_failed')
}

async function verifyToken(token: string) {
  try {
    const res = await fetch(apiUrl('/v1/users/verify'), {
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

const CIRCLE_COLORS = ['#E7FFDF', '#D06464', '#FFF4ED', '#FFCAAE', '#539EFF', '#8AE9FF']
const TEXT_LABEL_CLASS: Record<number, string> = {
  0: 'circleLabelTop',
  60: 'circleLabelRight',
  120: 'circleLabelRight',
  180: 'circleLabelBot',
  240: 'circleLabelLeft',
  300: 'circleLabelLeft',
}

function PlayerCircles(props: { players: { user_id: number; name: string }[]; anim: boolean; eliminatedIds: number[]; eliminatedCurrent: number | null }) {
  const [, forceUpdate] = useState(0)
  useEffect(() => {
    const onResize = () => forceUpdate((n) => n + 1)
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  const r = (320 / 1095) * (window.innerHeight || 1080)
  const d = (122 / 1095) * (window.innerHeight || 1080)

  const circles = props.players.map((p, i) => {
    const angle = i * 60
    const rad = (angle * Math.PI) / 180
    const x = Math.sin(rad) * r
    const y = -Math.cos(rad) * r
    const isElim = props.eliminatedIds.includes(p.user_id)
    const isCurrentElim = props.eliminatedCurrent === p.user_id

    return (
      <div
        key={p.user_id}
        className={
          isCurrentElim
            ? 'pCircle pCircle--elim'
            : isElim
              ? 'pCircle pCircle--gone'
              : props.anim
                ? 'pCircle pCircle--show'
                : 'pCircle'
        }
        style={{
          transform: `translate(calc(-50% + ${x}px), calc(-50% + ${y}px)) scale(${isElim ? 0 : props.anim ? 1 : 0})`,
          width: d,
          height: d,
          background: CIRCLE_COLORS[i],
        }}
      >
        <span className={`circleLabel ${TEXT_LABEL_CLASS[angle] ?? ''} ${props.anim && !isElim ? 'circleLabel--show' : ''}`}>
          {p.name}
        </span>
      </div>
    )
  })

  return <div className="playerCircles">{circles}</div>
}

function Game(props: { toast: (message: string) => void; blocked: boolean }) {
  const [pressed, setPressed] = useState(false)
  const [tutorialPressed, setTutorialPressed] = useState(false)
  const [settingsPressed, setSettingsPressed] = useState(false)
  const [matching, setMatching] = useState(false)
  const [dots, setDots] = useState(1)
  const dotsTimerRef = useRef<number | null>(null)

  const pollTimerRef = useRef<number | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [connectedRoomID, setConnectedRoomID] = useState<string | null>(null)
  const [currentUserId, setCurrentUserId] = useState<number | null>(null)
  const [roleReveal, setRoleReveal] = useState<'HUMAN' | 'SPY' | null>(null)
  const [roleFadeIn, setRoleFadeIn] = useState(false)
  const [gameWaiting, setGameWaiting] = useState(false)
  const [startrailGrow, setStartrailGrow] = useState(false)
  const [players, setPlayers] = useState<{ user_id: number; name: string }[]>([])
  const [showCircles, setShowCircles] = useState(false)
  const [circlesAnim, setCirclesAnim] = useState(false)
  const circlesTimerRef = useRef<number | null>(null)
  const answerTimerRef = useRef<number | null>(null)
  const answerDotsRef = useRef<number | null>(null)

  const [answering, setAnswering] = useState(false)
  const [answerBanner, setAnswerBanner] = useState<'slideIn' | 'hold' | 'slideOut' | 'done' | 'none'>('none')
  const [bannerClass, setBannerClass] = useState('')
  const [answerBoxShow, setAnswerBoxShow] = useState(false)

  const [voting, setVoting] = useState(false)
  const [voteBanner, setVoteBanner] = useState<'slideIn' | 'hold' | 'slideOut' | 'done' | 'none'>('none')
  const [voteBannerClass, setVoteBannerClass] = useState('')
  const [voteBoxShow, setVoteBoxShow] = useState(false)
  const [roundAnswers, setRoundAnswers] = useState<{ user_id: number; name: string; content: string }[]>([])
  const [selectedVote, setSelectedVote] = useState<number | null>(null)
  const [voteSent, setVoteSent] = useState(false)
  const [voteDots, setVoteDots] = useState(1)
  const [eliminatedIds, setEliminatedIds] = useState<number[]>([])
  const [eliminatedCurrent, setEliminatedCurrent] = useState<number | null>(null)
  const [gameOver, setGameOver] = useState(false)
  const [gameOverSVG, setGameOverSVG] = useState('')
  const [gameOverShow, setGameOverShow] = useState(false)
  const [storedRole, setStoredRole] = useState('')
  const storedRoleRef = useRef('')
  const [gamePhase, setGamePhase] = useState<'none' | 'fadeOut'>('none')

  useEffect(() => {
    const uid = localStorage.getItem(USER_ID_KEY)
    if (uid && !currentUserId) setCurrentUserId(Number(uid))
  }, [currentUserId])
  const voteTimerRef = useRef<number | null>(null)
  const voteDotsRef = useRef<number | null>(null)
  const [answerText, setAnswerText] = useState('')
  const [answerSent, setAnswerSent] = useState(false)
  const [answerDots, setAnswerDots] = useState(1)
  const [currentQuestion, setCurrentQuestion] = useState('')

  useEffect(() => {
    return () => {
      if (pollTimerRef.current != null) window.clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
      if (wsRef.current) wsRef.current.close()
      wsRef.current = null
      if (circlesTimerRef.current != null) window.clearTimeout(circlesTimerRef.current)
      if (answerTimerRef.current != null) window.clearTimeout(answerTimerRef.current)
      if (answerDotsRef.current != null) window.clearInterval(answerDotsRef.current)
      if (voteTimerRef.current != null) window.clearTimeout(voteTimerRef.current)
      if (voteDotsRef.current != null) window.clearInterval(voteDotsRef.current)
    }
  }, [])

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

  useEffect(() => {
    if (answerSent) {
      answerDotsRef.current = window.setInterval(() => {
        setAnswerDots((d) => (d % 3) + 1)
      }, 450)
      return () => {
        if (answerDotsRef.current != null) window.clearInterval(answerDotsRef.current)
      }
    }
  }, [answerSent])

  useEffect(() => {
    if (voteSent) {
      voteDotsRef.current = window.setInterval(() => {
        setVoteDots((d) => (d % 3) + 1)
      }, 450)
      return () => {
        if (voteDotsRef.current != null) window.clearInterval(voteDotsRef.current)
      }
    }
  }, [voteSent])

  useEffect(() => {
    if (voteBanner === 'done' && !voteBoxShow) {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => setVoteBoxShow(true))
      })
    }
  }, [voteBanner, voteBoxShow])

  const onPressStart = () => {
    if (props.blocked || matching || answerSent || voteSent || (currentUserId && eliminatedIds.includes(currentUserId))) return
    setPressed(true)
  }

  const onPressEnd = () => {
    setPressed(false)
  }

  const onTutorialPressStart = () => {
    if (props.blocked) return
    setTutorialPressed(true)
  }

  const onTutorialPressEnd = () => {
    setTutorialPressed(false)
  }

  const onSettingsPressStart = () => {
    if (props.blocked) return
    setSettingsPressed(true)
  }

  const onSettingsPressEnd = () => {
    setSettingsPressed(false)
  }

  const submit = async () => {
    if (props.blocked || matching || connectedRoomID) return

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
      // Backend currently expects {matchid} to be the username (not the numeric matchID).
      await startGame(name, token)

      if (pollTimerRef.current != null) window.clearInterval(pollTimerRef.current)
      pollTimerRef.current = window.setInterval(async () => {
        try {
          const s = await getMatchStatus(name, token)
          if (s.status !== 'IN_GAME') return
          if (!s.roomID) throw new Error('missing_room_id')

          window.clearInterval(pollTimerRef.current!)
          pollTimerRef.current = null
          setMatching(false)

          const ws = new WebSocket(toWsUrl(s.roomID, token))
          wsRef.current = ws
          setConnectedRoomID(s.roomID)

          ws.addEventListener('message', (e) => {
            try {
              const msg = JSON.parse(e.data)
              if (msg.action === 'game_started') {
                const role = msg.content?.your_role as string | undefined
                const pls = msg.content?.players as { user_id: number; name: string }[] | undefined
                if (pls) {
                  setPlayers(pls)
                }
                if (!storedRole && role) { storedRoleRef.current = role; setStoredRole(role) }
                if (role === 'HUMAN' || role === 'SPY') {
                  setRoleReveal(role)
                  requestAnimationFrame(() => {
                    requestAnimationFrame(() => setRoleFadeIn(true))
                  })
                  window.setTimeout(() => setRoleFadeIn(false), 5500)
                  window.setTimeout(() => setRoleReveal(null), 6000)
                }
              }
              if (msg.action === 'phase_change') {
                const phase = msg.content?.phase as string | undefined
                if (phase === 'WAITING') {
                  setGameWaiting(true)
                  setVoting(false)
                  setVoteBoxShow(false)
                  setVoteBanner('none')
                  setVoteSent(false)
                  setStartrailGrow(false)
                  requestAnimationFrame(() => {
                    requestAnimationFrame(() => setStartrailGrow(true))
                  })
                  circlesTimerRef.current = window.setTimeout(() => {
                    setShowCircles(true)
                    setEliminatedCurrent(null)
                    requestAnimationFrame(() => {
                      requestAnimationFrame(() => setCirclesAnim(true))
                    })
                  }, 1550)
                }
                if (phase === 'VOTE') {
                  setVoting(true)
                  setAnswering(false)
                  setAnswerBoxShow(false)
                  setAnswerBanner('none')
                  setAnswerSent(false)
                  voteTimerRef.current = window.setTimeout(() => {
                    setVoteBannerClass('')
                    setVoteBanner('slideIn')
                    requestAnimationFrame(() => {
                      setVoteBannerClass('speakBanner--slideIn')
                    })
                    voteTimerRef.current = window.setTimeout(() => {
                      setVoteBannerClass('speakBanner--hold')
                      setVoteBanner('hold')
                    }, 500)
                    voteTimerRef.current = window.setTimeout(() => {
                      setVoteBannerClass('speakBanner--slideOut')
                      setVoteBanner('slideOut')
                    }, 1500)
                    voteTimerRef.current = window.setTimeout(() => {
                      setVoteBanner('done')
                      requestAnimationFrame(() => {
                        requestAnimationFrame(() => setVoteBoxShow(true))
                      })
                    }, 2000)
                  }, 500)
                }
                if (phase === 'ANSWER') {
                  setAnswering(true)
                  setVoting(false)
                  setVoteBanner('none')
                  setVoteBoxShow(false)
                  setCirclesAnim(false)
                  setStartrailGrow(false)
                  answerTimerRef.current = window.setTimeout(() => {
                    setShowCircles(false)
                    setGameWaiting(false)
                    // Render banner off-screen first, then slide in.
                    setBannerClass('')
                    setAnswerBanner('slideIn')
                    requestAnimationFrame(() => {
                      setBannerClass('speakBanner--slideIn')
                    })
                    answerTimerRef.current = window.setTimeout(() => {
                      setBannerClass('speakBanner--hold')
                      setAnswerBanner('hold')
                    }, 500)
                    answerTimerRef.current = window.setTimeout(() => {
                      setBannerClass('speakBanner--slideOut')
                      setAnswerBanner('slideOut')
                    }, 1500)
                    answerTimerRef.current = window.setTimeout(() => {
                      setAnswerBanner('done')
                      requestAnimationFrame(() => {
                        requestAnimationFrame(() => setAnswerBoxShow(true))
                      })
                    }, 2000)
                  }, 500)
                }
              }
              if (msg.action === 'game_over') {
                const winner = msg.content?.winner as string | undefined
                setGameOverSVG(winner === 'SPY' ? spyWinSvg : winner === 'HUMAN' ? humanWinSvg : aiWinSvg)
                // Phase 1: fade out everything except background
                setGamePhase('fadeOut')
                window.setTimeout(() => {
                  // Phase 2: clear all UI, show settlement
                  setVoting(false)
                  setAnswering(false)
                  setGameWaiting(false)
                  setShowCircles(false)
                  setAnswerBanner('none')
                  setVoteBanner('none')
                  setAnswerBoxShow(false)
                  setVoteBoxShow(false)
                  setGameOver(true)
                  setGameOverShow(false)
                  requestAnimationFrame(() => {
                    requestAnimationFrame(() => setGameOverShow(true))
                  })
                }, 500)
              }
              if (msg.action === 'round_result') {
                const eid = msg.content?.eliminated_id as number | undefined
                if (eid && eid !== 0) {
                  setEliminatedIds((prev) => [...prev, eid])
                  setEliminatedCurrent(eid)
                }
              }
              if (msg.action === 'all_answers') {
                const ans = msg.content?.answers as { user_id: number; name: string; content: string }[] | undefined
                if (ans) setRoundAnswers(ans)
              }
              if (msg.action === 'question') {
                const q = msg.content?.question as string | undefined
                if (q) setCurrentQuestion(q)
              }
            } catch {
              // ignore
            }
          })
          ws.addEventListener('close', () => {
            wsRef.current = null
            setConnectedRoomID(null)
          })
        } catch {
          if (pollTimerRef.current != null) window.clearInterval(pollTimerRef.current)
          pollTimerRef.current = null
          setMatching(false)
          props.toast('匹配失败')
        }
      }, 1000)
    } catch {
      setMatching(false)
      props.toast('匹配失败')
    }
  }

  const submitVote = () => {
    if (!wsRef.current || voteSent || selectedVote == null) return
    setVoteSent(true)
    wsRef.current.send(JSON.stringify({ action: 'vote', target_user_id: selectedVote }))
  }

  const submitAnswer = () => {
    if (!wsRef.current || answerSent || !answerText.trim()) return
    setAnswerSent(true)
    wsRef.current.send(JSON.stringify({ action: 'answer', content: answerText.trim() }))
  }

  return (
    <section className={`game ${gamePhase === 'fadeOut' ? 'screenFade' : ''}`} aria-label="game">
      <img className="gameBg" src={gameBgSvg} alt="" aria-hidden="true" />
      {!gameOver && <div className="gameBottom" aria-hidden="true" />}
      {!gameOver && (
      <button
        className={pressed ? 'enterBtn enterBtn--pressed' : 'enterBtn'}
        type="button"
        onPointerDown={onPressStart}
        onPointerUp={onPressEnd}
        onPointerCancel={onPressEnd}
        onPointerLeave={onPressEnd}
        onClick={voting && voteBanner === 'done' ? submitVote : answering && answerBanner === 'done' ? submitAnswer : submit}
        disabled={currentUserId && eliminatedIds.includes(currentUserId) ? true : voting && voteBanner === 'done' ? voteSent : answering && answerBanner === 'done' ? answerSent : props.blocked || matching || !!connectedRoomID}
      >
        <span className="enterBtnInner">
          {voteSent
            ? `Waiting${'.'.repeat(voteDots)}`
            : answerSent
              ? `Waiting${'.'.repeat(answerDots)}`
                : voting && voteBanner === 'done'
                  ? 'Vote'
                  : voting
                    ? ''
                    : answering && answerBanner === 'done'
                      ? 'Commit'
                      : gameWaiting
                        ? ''
                        : connectedRoomID
                          ? 'Ready'
                          : matching
                            ? `Matching${'.'.repeat(dots)}`
                            : 'Enter'}
        </span>
      </button>
      )}

      {!gameOver && (
      <button
        className={tutorialPressed ? 'tutorialBtn tutorialBtn--pressed' : 'tutorialBtn'}
        type="button"
        onPointerDown={onTutorialPressStart}
        onPointerUp={onTutorialPressEnd}
        onPointerCancel={onTutorialPressEnd}
        onPointerLeave={onTutorialPressEnd}
        disabled={props.blocked}
        aria-label="tutorial"
      >
        <img className="tutorialIcon" src={tutorialSvg} alt="" aria-hidden="true" />
      </button>
      )}

      {!gameOver && (
      <button
        className={settingsPressed ? 'settingsBtn settingsBtn--pressed' : 'settingsBtn'}
        type="button"
        onPointerDown={onSettingsPressStart}
        onPointerUp={onSettingsPressEnd}
        onPointerCancel={onSettingsPressEnd}
        onPointerLeave={onSettingsPressEnd}
        disabled={props.blocked}
        aria-label="settings"
      >
        <img className="settingsIcon" src={settingsSvg} alt="" aria-hidden="true" />
      </button>
      )}
      {roleReveal && (
        <div className={roleFadeIn ? 'roleOverlay roleOverlay--show' : 'roleOverlay'} aria-hidden="true">
          <img className="roleSvg" src={roleReveal === 'HUMAN' ? humanSvg : spySvg} alt="" />
        </div>
      )}

      {gameWaiting && (
        <div className="startrailWrap" aria-hidden="true">
          <img
            className={startrailGrow ? 'startrailImg startrailImg--grow' : 'startrailImg'}
            src={startrailSvg}
            alt=""
          />
        </div>
      )}

      {showCircles && (
        <PlayerCircles players={players} anim={circlesAnim} eliminatedIds={eliminatedIds} eliminatedCurrent={eliminatedCurrent} />
      )}

      {answerBanner !== 'none' && answerBanner !== 'done' && (
        <div className={`speakBanner ${bannerClass}`} aria-hidden="true">
          <img className="speakBannerImg" src={speakSvg} alt="" />
        </div>
      )}

      {voting && voteBanner !== 'none' && voteBanner !== 'done' && (
        <div className={`speakBanner ${voteBannerClass}`} aria-hidden="true">
          <img className="speakBannerImg" src={verdictSvg} alt="" />
        </div>
      )}

      {voting && voteBanner === 'done' && voteBoxShow && (
        <div className="voteGrid" aria-label="vote">
          {roundAnswers.map((a) => {
            const idx = players.findIndex((p) => p.user_id === a.user_id)
            const color = CIRCLE_COLORS[idx >= 0 ? idx : 0]
            const sel = selectedVote === a.user_id
            return (
              <div key={a.user_id} className={`voteCard ${sel && !voteSent ? 'voteCard--sel' : ''}`}>
                <div className="voteCardTop">
                  <button
                    className="voteCircle"
                    style={{ background: color }}
                    type="button"
                    disabled={voteSent}
                    onClick={() => setSelectedVote(sel ? null : a.user_id)}
                  >
                  </button>
                  <span className="voteName">{a.name}</span>
                </div>
                <div className="voteContent">{a.content}</div>
              </div>
            )
          })}
        </div>
      )}

      {answering && answerBanner === 'done' && (
        <div className={answerBoxShow ? 'answerBox answerBox--show' : 'answerBox'} aria-label="answer input">
          <div className="answerBoxQuestion">{currentQuestion}</div>
          <div className="answerBoxDivider" />
          <textarea
            className={`answerBoxInput ${answerSent ? 'answerBoxInput--sent' : ''}`}
            value={answerText}
            onChange={(e) => setAnswerText(e.target.value)}
            disabled={answerSent}
            placeholder="输入你的回答..."
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                submitAnswer()
              }
            }}
          />
        </div>
      )}
      {gameOver && (
        <div className={gameOverShow ? 'gameOverOverlay gameOverOverlay--show' : 'gameOverOverlay'} aria-hidden="true">
          <img className="gameOverImg" src={gameOverSVG} alt="" />
        </div>
      )}
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
      const result = await login(username.trim(), password)
      localStorage.setItem(TOKEN_KEY, result.token)
      localStorage.setItem(USERNAME_KEY, username.trim())
      localStorage.setItem(USER_ID_KEY, String(result.id))
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
          >注册</button>
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

    const onKeyDown = (e: KeyboardEvent) => {
      // Backdoor: go straight to game from splash.
      if (view === 'splash' && e.key === 'Backspace') {
        transitionTo('game')
        return
      }
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
        {view === 'splash' && (
          <div className="splashHint">按任意键进入游戏......</div>
        )}
      </main>
      <Fade on={transitionOn} />
      {toastMessage && <Toast message={toastMessage} />}
    </>
  )
}
