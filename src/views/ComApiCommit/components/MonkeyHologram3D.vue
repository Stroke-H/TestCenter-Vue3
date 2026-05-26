<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as THREE from 'three'

interface MonkeyGraphNode {
  id: string
  title: string
  event: string
  activity: string
  risk: 'normal' | 'warning' | 'critical'
  imageUrl: string
  x: number
  y: number
}

interface MonkeyGraphEdge {
  source: string
  target: string
  event: string
}

const props = defineProps<{
  nodes: MonkeyGraphNode[]
  edges: MonkeyGraphEdge[]
  selectedId?: string
  targetApp?: string
}>()

const emit = defineEmits<{
  select: [node: MonkeyGraphNode]
}>()

const canvasHost = ref<HTMLElement | null>(null)

let renderer: THREE.WebGLRenderer | null = null
let scene: THREE.Scene | null = null
let camera: THREE.PerspectiveCamera | null = null
let animationId = 0
let graphGroup: THREE.Group | null = null
let nodeMeshes: Array<THREE.Mesh & { userData: { node: MonkeyGraphNode } }> = []
let raycaster: THREE.Raycaster | null = null
let pointer: THREE.Vector2 | null = null
let isDragging = false
let lastPointer = { x: 0, y: 0 }
let targetRotation = { x: -0.22, y: 0.34 }
let currentRotation = { x: -0.22, y: 0.34 }

const riskColors: Record<MonkeyGraphNode['risk'], number> = {
  normal: 0x28f0a0,
  warning: 0xffb020,
  critical: 0xff476f
}

// -------------------------------------------------------------
// Sci-Fi Tech Hologram Elements
// -------------------------------------------------------------
let gridHelper: THREE.GridHelper | null = null
let dustParticles: THREE.Points | null = null
const dustCount = 350

interface NodeAssembly {
  group: THREE.Group
  core: THREE.Mesh
  wireframeShell: THREE.Mesh
  electronRing: THREE.Mesh
  glowHalo: THREE.Sprite
  label: THREE.Sprite
  smallOrbit: THREE.LineLoop
  node: MonkeyGraphNode
}
let nodeAssemblies: NodeAssembly[] = []

interface DataPacket {
  mesh: THREE.Mesh
  source: THREE.Vector3
  target: THREE.Vector3
  progress: number
  speed: number
}
let dataPackets: DataPacket[] = []

let selectedRing: THREE.LineLoop | null = null
let orbitsGroup: THREE.Group | null = null
let centerRings: THREE.LineLoop[] = []
let globeShell: THREE.Mesh | null = null

const disposeObject = (object: THREE.Object3D) => {
  object.traverse((child) => {
    const mesh = child as THREE.Mesh
    if (mesh.geometry) mesh.geometry.dispose()
    const material = mesh.material
    if (Array.isArray(material)) {
      material.forEach((item) => item.dispose())
    } else if (material) {
      material.dispose()
    }
  })
}

// Layout: Fibonacci sphere, so the whole model reads as a real globe instead of flat rings.
const nodeToPosition = (_node: MonkeyGraphNode, index: number) => {
  const total = Math.max(props.nodes.length, 1)
  const goldenAngle = Math.PI * (3 - Math.sqrt(5))
  const normalized = total === 1 ? 0 : index / (total - 1)
  const unitY = 1 - normalized * 2
  const radiusAtY = Math.sqrt(Math.max(0, 1 - unitY * unitY))
  const theta = index * goldenAngle
  const sphereRadius = Math.min(7.2, Math.max(4.6, 3.8 + Math.sqrt(total) * 0.36))
  const shellOffset = ((index % 5) - 2) * 0.08
  const x = Math.cos(theta) * radiusAtY * (sphereRadius + shellOffset)
  const z = Math.sin(theta) * radiusAtY * (sphereRadius + shellOffset)
  const y = unitY * (sphereRadius + shellOffset)
  
  return new THREE.Vector3(x, y, z)
}

const createCenterAppTexture = (targetApp: string) => {
  const canvas = document.createElement('canvas')
  canvas.width = 256
  canvas.height = 256
  const ctx = canvas.getContext('2d')
  if (ctx) {
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    
    // Draw outer glow circle
    const grad = ctx.createRadialGradient(128, 128, 60, 128, 128, 120)
    grad.addColorStop(0, 'rgba(0, 240, 255, 0.15)')
    grad.addColorStop(0.8, 'rgba(0, 240, 255, 0.75)')
    grad.addColorStop(1, 'rgba(0, 240, 255, 0)')
    ctx.fillStyle = grad
    ctx.beginPath()
    ctx.arc(128, 128, 120, 0, Math.PI * 2)
    ctx.fill()
    
    // Draw solid inner circle background
    ctx.fillStyle = '#020617'
    ctx.strokeStyle = '#00f0ff'
    ctx.lineWidth = 4
    ctx.beginPath()
    ctx.arc(128, 128, 70, 0, Math.PI * 2)
    ctx.fill()
    ctx.stroke()
    
    // Draw outer dotted circle
    ctx.strokeStyle = 'rgba(0, 240, 255, 0.35)'
    ctx.lineWidth = 2
    ctx.setLineDash([4, 8])
    ctx.beginPath()
    ctx.arc(128, 128, 90, 0, Math.PI * 2)
    ctx.stroke()
    ctx.setLineDash([])
    
    // Draw APP symbol in the center
    const normalizedApp = targetApp.toLowerCase()
    const isWave = normalizedApp.includes('wave') || normalizedApp.includes('short')
    const isNovel = normalizedApp.includes('novel') || normalizedApp.includes('read')
    
    ctx.strokeStyle = '#00f0ff'
    ctx.fillStyle = '#00f0ff'
    ctx.lineWidth = 3
    
    if (isWave) {
      // Wave / Play symbol
      ctx.beginPath()
      ctx.moveTo(115, 103)
      ctx.lineTo(145, 128)
      ctx.lineTo(115, 153)
      ctx.closePath()
      ctx.fill()
      
      ctx.beginPath()
      ctx.arc(128, 128, 48, -Math.PI / 3, Math.PI / 3)
      ctx.stroke()
      ctx.beginPath()
      ctx.arc(128, 128, 56, -Math.PI / 3, Math.PI / 3)
      ctx.stroke()
    } else if (isNovel) {
      // Book symbol
      ctx.beginPath()
      ctx.moveTo(128, 145)
      ctx.lineTo(128, 110)
      ctx.quadraticCurveTo(110, 105, 98, 115)
      ctx.lineTo(98, 150)
      ctx.quadraticCurveTo(110, 140, 128, 145)
      ctx.fill()
      
      ctx.beginPath()
      ctx.moveTo(128, 145)
      ctx.lineTo(128, 110)
      ctx.quadraticCurveTo(146, 105, 158, 115)
      ctx.lineTo(158, 150)
      ctx.quadraticCurveTo(146, 140, 128, 145)
      ctx.fill()
    } else {
      ctx.beginPath()
      ctx.arc(128, 128, 30, 0, Math.PI * 2)
      ctx.stroke()
      ctx.beginPath()
      ctx.moveTo(128, 85)
      ctx.lineTo(128, 171)
      ctx.moveTo(85, 128)
      ctx.lineTo(171, 128)
      ctx.stroke()
    }
    
    ctx.font = 'bold 12px Courier New, monospace'
    ctx.fillStyle = '#a5f3fc'
    ctx.textAlign = 'center'
    const nameToShow = isWave ? 'SHORTSWAVE' : isNovel ? 'NOVELNOVA' : 'TEST TARGET'
    ctx.fillText(nameToShow, 128, 185)
    ctx.font = '8px Courier New, monospace'
    ctx.fillStyle = '#64748b'
    ctx.fillText('ADB MONKEY AGENT', 128, 200)
  }
  return new THREE.CanvasTexture(canvas)
}

const createGlowSprite = (color: number, scale: number) => {
  const canvas = document.createElement('canvas')
  canvas.width = 128
  canvas.height = 128
  const ctx = canvas.getContext('2d')
  if (ctx) {
    const gradient = ctx.createRadialGradient(64, 64, 6, 64, 64, 64)
    gradient.addColorStop(0, '#ffffff')
    gradient.addColorStop(0.18, `#${color.toString(16).padStart(6, '0')}`)
    gradient.addColorStop(1, 'rgba(0,0,0,0)')
    ctx.fillStyle = gradient
    ctx.fillRect(0, 0, 128, 128)
  }
  const texture = new THREE.CanvasTexture(canvas)
  const material = new THREE.SpriteMaterial({
    map: texture,
    blending: THREE.AdditiveBlending,
    depthWrite: false,
    transparent: true,
    opacity: 0.65
  })
  const sprite = new THREE.Sprite(material)
  sprite.scale.set(scale, scale, 1)
  return sprite
}

const createLabelSprite = (node: MonkeyGraphNode, pos: THREE.Vector3) => {
  const canvas = document.createElement('canvas')
  canvas.width = 256
  canvas.height = 96
  const ctx = canvas.getContext('2d')
  if (ctx) {
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    
    ctx.fillStyle = 'rgba(2, 6, 23, 0.78)'
    ctx.strokeStyle = node.risk === 'critical' ? 'rgba(255, 71, 111, 0.65)' : 'rgba(0, 240, 255, 0.55)'
    ctx.lineWidth = 2
    
    const w = canvas.width
    const h = canvas.height
    const r = 8
    
    ctx.beginPath()
    ctx.moveTo(r, 4)
    ctx.lineTo(w - r, 4)
    ctx.quadraticCurveTo(w - 4, 4, w - 4, r)
    ctx.lineTo(w - 4, h - r)
    ctx.quadraticCurveTo(w - 4, h - 4, w - r, h - 4)
    ctx.lineTo(r, h - 4)
    ctx.quadraticCurveTo(4, h - 4, 4, h - r)
    ctx.lineTo(4, r)
    ctx.quadraticCurveTo(4, 4, r, 4)
    ctx.closePath()
    ctx.fill()
    ctx.stroke()
    
    ctx.strokeStyle = node.risk === 'critical' ? '#ff476f' : '#00f0ff'
    ctx.lineWidth = 3
    
    ctx.beginPath()
    ctx.moveTo(4, 16)
    ctx.lineTo(4, 4)
    ctx.lineTo(16, 4)
    ctx.stroke()
    
    ctx.beginPath()
    ctx.moveTo(w - 4, h - 16)
    ctx.lineTo(w - 4, h - 4)
    ctx.lineTo(w - 16, h - 4)
    ctx.stroke()
    
    ctx.font = 'bold 15px Courier New, monospace'
    ctx.fillStyle = '#ffffff'
    ctx.fillText(`SYS: ${node.id}`, 16, 26)
    
    ctx.font = '11px Courier New, monospace'
    ctx.fillStyle = '#a5f3fc'
    ctx.fillText(`ACT: ${node.activity.substring(0, 20)}`, 16, 46)
    
    const riskStr = `RISK: ${node.risk.toUpperCase()}`
    const coordStr = `LOC: [${pos.x.toFixed(1)}, ${pos.y.toFixed(1)}, ${pos.z.toFixed(1)}]`
    
    ctx.fillStyle = node.risk === 'critical' ? '#ff476f' : node.risk === 'warning' ? '#ffb020' : '#28f0a0'
    ctx.fillText(riskStr, 16, 64)
    
    ctx.fillStyle = '#64748b'
    ctx.fillText(coordStr, 16, 80)
  }
  const texture = new THREE.CanvasTexture(canvas)
  const material = new THREE.SpriteMaterial({ map: texture, transparent: true, depthWrite: false })
  const sprite = new THREE.Sprite(material)
  sprite.scale.set(1.15, 0.43, 1)
  return sprite
}

const createOrbit = (radius: number, rotation: [number, number, number], color: number, isDashed = false) => {
  const curve = new THREE.EllipseCurve(0, 0, radius, radius * 0.58, 0, Math.PI * 2)
  const points = curve.getPoints(180).map((point) => new THREE.Vector3(point.x, point.y, 0))
  const geometry = new THREE.BufferGeometry().setFromPoints(points)
  
  let material: THREE.Material
  if (isDashed) {
    material = new THREE.LineDashedMaterial({
      color,
      transparent: true,
      opacity: 0.4,
      dashSize: 0.2,
      gapSize: 0.1,
      blending: THREE.AdditiveBlending
    })
  } else {
    material = new THREE.LineBasicMaterial({
      color,
      transparent: true,
      opacity: 0.32,
      blending: THREE.AdditiveBlending
    })
  }
  
  const line = new THREE.LineLoop(geometry, material)
  line.rotation.set(...rotation)
  if (isDashed) {
    line.computeLineDistances()
  }
  return line
}

const rebuildGraph = () => {
  if (!scene) return
  if (graphGroup) {
    scene.remove(graphGroup)
    disposeObject(graphGroup)
  }

  const group = new THREE.Group()
  graphGroup = group
  nodeMeshes = []
  nodeAssemblies = []
  dataPackets = []

  // Create tech orbits group
  orbitsGroup = new THREE.Group()
  orbitsGroup.add(createOrbit(5.6, [1.57, 0.08, 0.2], 0x00f0ff, false))
  orbitsGroup.add(createOrbit(5.8, [0.78, 0.92, -0.36], 0xa78bfa, true))
  orbitsGroup.add(createOrbit(5.4, [2.08, -0.74, 0.82], 0x34d399, false))
  group.add(orbitsGroup)

  const shellGeometry = new THREE.SphereGeometry(5.85, 48, 32)
  const shellMaterial = new THREE.MeshBasicMaterial({
    color: 0x67e8f9,
    wireframe: true,
    transparent: true,
    opacity: 0.035,
    blending: THREE.AdditiveBlending,
    depthWrite: false
  })
  globeShell = new THREE.Mesh(shellGeometry, shellMaterial)
  group.add(globeShell)

  const positions = new Map<string, THREE.Vector3>()
  props.nodes.forEach((node, index) => {
    positions.set(node.id, nodeToPosition(node, index))
  })

  // Create connection edges lines
  props.edges.forEach((edge) => {
    const source = positions.get(edge.source)
    const target = positions.get(edge.target)
    if (!source || !target) return
    const geometry = new THREE.BufferGeometry().setFromPoints([source, target])
    const material = new THREE.LineBasicMaterial({
      color: 0x00d0ff,
      transparent: true,
      opacity: 0.25,
      blending: THREE.AdditiveBlending
    })
    group.add(new THREE.Line(geometry, material))
  })

  // Create center APP logo node at (0, 0, 0)
  const centerAppTex = createCenterAppTexture(props.targetApp || 'com.company.shortsdrama.wave')
  const centerAppMat = new THREE.SpriteMaterial({
    map: centerAppTex,
    transparent: true,
    blending: THREE.AdditiveBlending
  })
  const centerSprite = new THREE.Sprite(centerAppMat)
  centerSprite.scale.set(1.55, 1.55, 1)
  centerSprite.position.set(0, 0, 0)
  group.add(centerSprite)

  // Rotating concentric rings around center
  centerRings = []
  const cRing1 = createOrbit(1.0, [1.1, 0.4, 0.1], 0x00f0ff, true)
  const cRing2 = createOrbit(1.45, [1.3, -0.2, -0.3], 0xa78bfa, true)
  group.add(cRing1)
  group.add(cRing2)
  centerRings.push(cRing1, cRing2)

  // Connect all nodes to center and stream data packets
  const centerPos = new THREE.Vector3(0, 0, 0)
  positions.forEach((position) => {
    const geometry = new THREE.BufferGeometry().setFromPoints([position, centerPos])
    const material = new THREE.LineBasicMaterial({
      color: 0x00f0ff,
      transparent: true,
      opacity: 0.12,
      blending: THREE.AdditiveBlending
    })
    group.add(new THREE.Line(geometry, material))
    
    // Low-density data packets prevent clutter when many screenshots exist.
    if (Math.random() > 0.82) {
      const packetGeo = new THREE.SphereGeometry(0.038, 6, 6)
      const packetMat = new THREE.MeshBasicMaterial({
        color: 0x00ffcc,
        transparent: true,
        opacity: 0.75,
        blending: THREE.AdditiveBlending
      })
      const packetMesh = new THREE.Mesh(packetGeo, packetMat)
      group.add(packetMesh)
      
      dataPackets.push({
        mesh: packetMesh,
        source: position.clone(),
        target: centerPos.clone(),
        progress: Math.random(),
        speed: 0.002 + Math.random() * 0.003
      })
    }
  })

  // Create selection concentric shockwave ripple ring
  const circleCurve = new THREE.EllipseCurve(0, 0, 0.8, 0.8, 0, Math.PI * 2)
  const circlePoints = circleCurve.getPoints(64).map(p => new THREE.Vector3(p.x, p.y, 0))
  const circleGeo = new THREE.BufferGeometry().setFromPoints(circlePoints)
  const circleMat = new THREE.LineBasicMaterial({
    color: 0xff00a0,
    transparent: true,
    opacity: 0,
    blending: THREE.AdditiveBlending
  })
  selectedRing = new THREE.LineLoop(circleGeo, circleMat)
  selectedRing.visible = false
  group.add(selectedRing)

  // Create Node assemblies
  props.nodes.forEach((node, index) => {
    const color = riskColors[node.risk]
    const position = positions.get(node.id) || new THREE.Vector3()
    
    const nodeGroup = new THREE.Group()
    nodeGroup.position.copy(position)
    
    // 1. Glowing white inner core
    const coreGeo = new THREE.SphereGeometry(node.risk === 'critical' ? 0.07 : 0.052, 14, 14)
    const coreMat = new THREE.MeshBasicMaterial({
      color: 0xffffff,
      transparent: true,
      opacity: 0.95
    })
    const core = new THREE.Mesh(coreGeo, coreMat)
    nodeGroup.add(core)
    
    // 2. Translucent outer wireframe shell (icosahedron)
    const shellGeo = new THREE.IcosahedronGeometry(node.risk === 'critical' ? 0.24 : 0.18, 1)
    const shellMat = new THREE.MeshBasicMaterial({
      color,
      wireframe: true,
      transparent: true,
      opacity: 0.45,
      blending: THREE.AdditiveBlending
    })
    const wireframeShell = new THREE.Mesh(shellGeo, shellMat)
    nodeGroup.add(wireframeShell)
    
    // 3. Electron Bohr Torus Ring
    const torusGeo = new THREE.TorusGeometry(node.risk === 'critical' ? 0.2 : 0.15, 0.007, 6, 24)
    const torusMat = new THREE.MeshBasicMaterial({
      color,
      transparent: true,
      opacity: 0.65,
      blending: THREE.AdditiveBlending
    })
    const electronRing = new THREE.Mesh(torusGeo, torusMat)
    // Tilted rotation
    electronRing.rotation.set(Math.random() * Math.PI, Math.random() * Math.PI, 0)
    nodeGroup.add(electronRing)
    
    // 4. Glow Halo Sprite
    const glowHalo = createGlowSprite(color, node.risk === 'critical' ? 1.35 : 1.0)
    nodeGroup.add(glowHalo)
    
    // 5. Sci-Fi coordinate Label
    const label = createLabelSprite(node, position)
    label.position.set(0, -0.35, 0)
    nodeGroup.add(label)
    
    // 6. Aesthetic small ring
    const smallOrbit = createOrbit(node.risk === 'critical' ? 0.32 : 0.25, [1.35, 0.3 + index * 0.1, 0.4], color, false)
    nodeGroup.add(smallOrbit)
    
    // 7. Invisible click box for raycasting (active but transparent)
    const hitBoxGeo = new THREE.SphereGeometry(0.28, 8, 8)
    const hitBoxMat = new THREE.MeshBasicMaterial({ transparent: true, opacity: 0 })
    const hitBox = new THREE.Mesh(hitBoxGeo, hitBoxMat) as unknown as THREE.Mesh & { userData: { node: MonkeyGraphNode } }
    hitBox.userData = { node }
    nodeGroup.add(hitBox)
    
    nodeMeshes.push(hitBox)
    group.add(nodeGroup)
    
    nodeAssemblies.push({
      group: nodeGroup,
      core,
      wireframeShell,
      electronRing,
      glowHalo,
      label,
      smallOrbit,
      node
    })
  })

  scene.add(group)
}

const updateSelectedState = () => {
  const positions = new Map<string, THREE.Vector3>()
  props.nodes.forEach((node, index) => {
    positions.set(node.id, nodeToPosition(node, index))
  })

  const selectedNode = props.nodes.find((n) => n.id === props.selectedId)
  if (selectedRing) {
    if (selectedNode) {
      const position = positions.get(selectedNode.id) || new THREE.Vector3()
      selectedRing.position.copy(position)
      selectedRing.visible = true
    } else {
      selectedRing.visible = false
    }
  }

  nodeAssemblies.forEach((assembly) => {
    const isSelected = assembly.node.id === props.selectedId
    const wireframeMat = assembly.wireframeShell.material as THREE.MeshBasicMaterial
    const torusMat = assembly.electronRing.material as THREE.MeshBasicMaterial
    
    if (isSelected) {
      assembly.group.scale.setScalar(1.18)
      wireframeMat.opacity = 0.9
      torusMat.opacity = 0.95
    } else {
      assembly.group.scale.setScalar(1.0)
      wireframeMat.opacity = 0.45
      torusMat.opacity = 0.65
    }
  })
}

const resizeRenderer = () => {
  if (!canvasHost.value || !renderer || !camera) return
  const { clientWidth, clientHeight } = canvasHost.value
  renderer.setSize(clientWidth, clientHeight)
  camera.aspect = clientWidth / Math.max(clientHeight, 1)
  camera.updateProjectionMatrix()
}

const handleWheel = (event: WheelEvent) => {
  if (!camera) return
  // Zoom in / out when holding Ctrl or pinching on macOS trackpad
  if (event.ctrlKey) {
    event.preventDefault()
    const zoomFactor = event.deltaY * 0.015
    camera.position.z = Math.max(5.5, Math.min(34, camera.position.z + zoomFactor))
  }
}

const animate = () => {
  if (!renderer || !scene || !camera || !graphGroup) return
  
  // Smoothly rotate the whole graph group
  currentRotation.x += (targetRotation.x - currentRotation.x) * 0.08
  currentRotation.y += (targetRotation.y - currentRotation.y) * 0.08
  graphGroup.rotation.x = currentRotation.x
  graphGroup.rotation.y = currentRotation.y
  graphGroup.rotation.z = Math.sin(performance.now() / 3200) * 0.06
  
  // Rotate orbits inside
  if (orbitsGroup && orbitsGroup.children.length >= 3) {
    const o0 = orbitsGroup.children[0]
    const o1 = orbitsGroup.children[1]
    const o2 = orbitsGroup.children[2]
    if (o0) o0.rotation.z += 0.001
    if (o1) o1.rotation.z -= 0.002
    if (o2) o2.rotation.z += 0.0015
  }

  if (globeShell) {
    globeShell.rotation.y -= 0.0008
    globeShell.rotation.x += 0.0003
  }

  // Rotate center rings
  centerRings.forEach((ring, index) => {
    ring.rotation.z += (index === 0 ? 0.005 : -0.003)
  })
  
  // Animate node assemblies
  nodeAssemblies.forEach((assembly, index) => {
    // Soft float offset
    const pulse = Math.sin(performance.now() * 0.0015 + index) * 0.08
    assembly.core.position.y = pulse * 0.2
    
    // Rotate shell & ring in opposite directions
    assembly.wireframeShell.rotation.y += 0.012
    assembly.wireframeShell.rotation.x += 0.006
    
    assembly.electronRing.rotation.z -= 0.025
    
    // Rotate aesthetic ring
    assembly.smallOrbit.rotation.z += 0.008
  })

  // Animate data packet flows along edges
  dataPackets.forEach((packet) => {
    packet.progress += packet.speed
    if (packet.progress > 1) {
      packet.progress = 0
    }
    packet.mesh.position.lerpVectors(packet.source, packet.target, packet.progress)
    const scalePulse = 1.0 + Math.sin(performance.now() * 0.015 + packet.progress * 8) * 0.3
    packet.mesh.scale.setScalar(scalePulse)
  })

  // Selected shockwave ripple ring animation
  if (selectedRing && selectedRing.visible) {
    const t = (performance.now() % 1400) / 1400
    selectedRing.scale.setScalar(0.4 + t * 1.6)
    const mat = selectedRing.material as THREE.LineBasicMaterial
    mat.opacity = Math.sin(t * Math.PI) * 0.85
    selectedRing.rotation.z += 0.01
  }

  // Floating background cyber-dust particles animation
  if (dustParticles) {
    const positionAttr = dustParticles.geometry.getAttribute('position') as THREE.BufferAttribute | undefined
    if (positionAttr && positionAttr.array) {
      const positions = positionAttr.array as Float32Array
      for (let i = 0; i < dustCount; i++) {
        const x = positions[i * 3]
        const y = positions[i * 3 + 1]
        const z = positions[i * 3 + 2]
        
        if (x !== undefined && y !== undefined && z !== undefined) {
          const newX = x + Math.sin(performance.now() * 0.0006 + i) * 0.002
          const newY = y + Math.cos(performance.now() * 0.0008 + i) * 0.002
          const newZ = z + Math.sin(performance.now() * 0.0005 + i) * 0.002
          
          positions[i * 3] = Math.abs(newX) > 14 ? -newX : newX
          positions[i * 3 + 1] = Math.abs(newY) > 10 ? -newY : newY
          positions[i * 3 + 2] = Math.abs(newZ) > 14 ? -newZ : newZ
        }
      }
      positionAttr.needsUpdate = true
    }
    dustParticles.rotation.y += 0.0003
  }

  // Slowly rotate the bottom grid helper
  if (gridHelper) {
    gridHelper.rotation.y = performance.now() * 0.00004
  }

  renderer.render(scene, camera)
  animationId = requestAnimationFrame(animate)
}

const handlePointerDown = (event: PointerEvent) => {
  isDragging = true
  lastPointer = { x: event.clientX, y: event.clientY }
}

const handlePointerMove = (event: PointerEvent) => {
  if (!isDragging) return
  const dx = event.clientX - lastPointer.x
  const dy = event.clientY - lastPointer.y
  targetRotation.y += dx * 0.006
  targetRotation.x += dy * 0.006
  lastPointer = { x: event.clientX, y: event.clientY }
}

const handlePointerUp = () => {
  isDragging = false
}

const handleClick = (event: MouseEvent) => {
  if (!canvasHost.value || !camera || !raycaster || !pointer) return
  const rect = canvasHost.value.getBoundingClientRect()
  pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1
  pointer.y = -(((event.clientY - rect.top) / rect.height) * 2 - 1)
  raycaster.setFromCamera(pointer, camera)
  const intersects = raycaster.intersectObjects(nodeMeshes, false)
  const selected = intersects[0]?.object as (THREE.Mesh & { userData: { node: MonkeyGraphNode } }) | undefined
  if (selected?.userData.node) emit('select', selected.userData.node)
}

const initScene = () => {
  if (!canvasHost.value) return
  scene = new THREE.Scene()
  scene.fog = new THREE.FogExp2(0x020617, 0.035)

  camera = new THREE.PerspectiveCamera(42, 1, 0.1, 100)
  camera.position.set(0, 0, 23)

  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.outputColorSpace = THREE.SRGBColorSpace
  canvasHost.value.appendChild(renderer.domElement)

  // Cyber-dust background particles creation
  const dustGeometry = new THREE.BufferGeometry()
  const dustPositions = new Float32Array(dustCount * 3)
  for (let i = 0; i < dustCount; i++) {
    dustPositions[i * 3] = (Math.random() - 0.5) * 28
    dustPositions[i * 3 + 1] = (Math.random() - 0.5) * 20
    dustPositions[i * 3 + 2] = (Math.random() - 0.5) * 28
  }
  dustGeometry.setAttribute('position', new THREE.BufferAttribute(dustPositions, 3))
  
  const createPointTexture = () => {
    const canvas = document.createElement('canvas')
    canvas.width = 16
    canvas.height = 16
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const gradient = ctx.createRadialGradient(8, 8, 0, 8, 8, 8)
      gradient.addColorStop(0, 'rgba(255, 255, 255, 1)')
      gradient.addColorStop(0.35, 'rgba(103, 232, 249, 0.85)')
      gradient.addColorStop(1, 'rgba(0, 0, 0, 0)')
      ctx.fillStyle = gradient
      ctx.fillRect(0, 0, 16, 16)
    }
    return new THREE.CanvasTexture(canvas)
  }

  const dustMaterial = new THREE.PointsMaterial({
    color: 0x67e8f9,
    size: 0.18,
    map: createPointTexture(),
    transparent: true,
    opacity: 0.55,
    blending: THREE.AdditiveBlending,
    depthWrite: false
  })
  dustParticles = new THREE.Points(dustGeometry, dustMaterial)
  scene.add(dustParticles)

  // Bottom high-tech grid helper
  gridHelper = new THREE.GridHelper(20, 20, 0x00f0ff, 0x1e293b)
  gridHelper.position.y = -7.2
  const gridMat = gridHelper.material as THREE.Material
  gridMat.transparent = true
  gridMat.opacity = 0.22
  gridMat.blending = THREE.AdditiveBlending
  scene.add(gridHelper)

  const ambient = new THREE.AmbientLight(0x88ccff, 0.9)
  scene.add(ambient)

  const keyLight = new THREE.PointLight(0x67e8f9, 3.2, 40)
  keyLight.position.set(4, 5, 8)
  scene.add(keyLight)

  const rimLight = new THREE.PointLight(0xa78bfa, 2.5, 38)
  rimLight.position.set(-6, -2, 8)
  scene.add(rimLight)

  raycaster = new THREE.Raycaster()
  pointer = new THREE.Vector2()
  
  canvasHost.value.addEventListener('wheel', handleWheel, { passive: false })
  
  resizeRenderer()
  rebuildGraph()
  updateSelectedState()
  animate()
}

onMounted(() => {
  initScene()
  window.addEventListener('resize', resizeRenderer)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(animationId)
  window.removeEventListener('resize', resizeRenderer)
  if (canvasHost.value) {
    canvasHost.value.removeEventListener('wheel', handleWheel)
  }
  
  if (graphGroup) {
    disposeObject(graphGroup)
    scene?.remove(graphGroup)
  }
  if (gridHelper) {
    disposeObject(gridHelper)
    scene?.remove(gridHelper)
  }
  if (dustParticles) {
    disposeObject(dustParticles)
    scene?.remove(dustParticles)
  }
  
  renderer?.dispose()
  renderer?.domElement.remove()
  renderer = null
  scene = null
  camera = null
  graphGroup = null
  gridHelper = null
  dustParticles = null
})

watch(() => [props.nodes, props.edges], () => {
  rebuildGraph()
  updateSelectedState()
}, { deep: true })

watch(() => props.selectedId, updateSelectedState)
</script>

<template>
  <div
    ref="canvasHost"
    class="monkey-hologram-3d"
    @pointerdown="handlePointerDown"
    @pointermove="handlePointerMove"
    @pointerup="handlePointerUp"
    @pointerleave="handlePointerUp"
    @click="handleClick"
  >
    <div class="monkey-hologram-3d__hud">
      <span class="pulse-indicator"></span>
      <span>HOLOGRAPHIC ATOM SCANNER</span>
      <span class="divider">//</span>
      <strong>ACTIVE NODE: {{ nodes.length }}</strong>
    </div>
  </div>
</template>

<style scoped>
.monkey-hologram-3d {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  cursor: grab;
  border-radius: 22px;
  background:
    radial-gradient(circle at 50% 44%, rgba(0, 240, 255, 0.12), transparent 32%),
    radial-gradient(circle at 18% 78%, rgba(99, 102, 241, 0.12), transparent 28%),
    linear-gradient(135deg, #020617 0%, #07111f 45%, #0e1726 100%);
}

.monkey-hologram-3d:active {
  cursor: grabbing;
}

.monkey-hologram-3d::before {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(rgba(0, 240, 255, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(0, 240, 255, 0.05) 1px, transparent 1px);
  background-size: 32px 32px;
  mask-image: radial-gradient(circle at center, #000 20%, transparent 78%);
}

.monkey-hologram-3d :deep(canvas) {
  position: relative;
  z-index: 1;
  display: block;
}

.monkey-hologram-3d__hud {
  position: absolute;
  left: 16px;
  top: 16px;
  z-index: 2;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: 4px;
  color: #cffafe;
  background: rgba(2, 6, 23, 0.72);
  border: 1px solid rgba(0, 240, 255, 0.35);
  backdrop-filter: blur(14px);
  font-family: 'Courier New', monospace;
  font-size: 11px;
  font-weight: bold;
  letter-spacing: 1px;
  box-shadow: 0 0 16px rgba(0, 240, 255, 0.15);
}

.pulse-indicator {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: #00f0ff;
  box-shadow: 0 0 8px #00f0ff;
  animation: hud-blink 1.5s infinite ease-in-out;
}

.divider {
  color: rgba(0, 240, 255, 0.4);
  font-weight: normal;
}

@keyframes hud-blink {
  0%, 100% { opacity: 0.35; }
  50% { opacity: 1; }
}

.monkey-hologram-3d__hud strong {
  color: #86efac;
}
</style>
