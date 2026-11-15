import { ASCII } from './constants';
import { fitKnot, gridSize } from './layout';
import { identity, multiply, normalMatrix3, perspective, rotateX, rotateY, scale, translate, type Mat4 } from './matrix';
import { ASCII_FS, QUAD_VS, SCENE_FS, SCENE_VS } from './shaders';
import { torusKnot } from './torusKnot';

export type FrameState = {
  /** Seconds of animation time. */
  t: number;
  /** Intro decode progress, 0..1. */
  intro: number;
  /** Sweep position, or -1 for off. */
  sweep: number;
  yaw: number;
  tiltX: number;
  tiltY: number;
  /** Pointer in CSS px from the canvas top-left, or null when outside. */
  pointer: [number, number] | null;
};

export type AsciiRenderer = {
  resize(cssW: number, cssH: number, dpr: number): void;
  frame(s: FrameState): void;
  dispose(): void;
};

type Program = { p: WebGLProgram; u: Record<string, WebGLUniformLocation | null> };
type Target = { tex: WebGLTexture; rb: WebGLRenderbuffer; fb: WebGLFramebuffer; w: number; h: number };

const RAMP = Array.from(ASCII.ramp);
const GLYPHS = [...RAMP, ...Array.from(ASCII.edges)];
const rgb = (c: readonly number[]) => new Float32Array(c.map((x) => x / 255));

/**
 * Creates the plain-WebGL ASCII renderer on `canvas`. Returns null when WebGL is
 * unavailable or any setup step fails. Never throws and never logs.
 * `onFail` fires if a later context restore or resize cannot be completed.
 */
export function createAsciiRenderer(canvas: HTMLCanvasElement, onFail?: () => void): AsciiRenderer | null {
  if (typeof window === 'undefined' || typeof window.WebGLRenderingContext === 'undefined') return null;
  try {
    const gl = canvas.getContext('webgl', { alpha: true, premultipliedAlpha: true, antialias: false, depth: false });
    if (!gl) return null;
    return build(canvas, gl as WebGLRenderingContext, onFail);
  } catch {
    return null;
  }
}

function build(canvas: HTMLCanvasElement, gl: WebGLRenderingContext, onFail?: () => void): AsciiRenderer {
  const knot = torusKnot(ASCII.knot.R, ASCII.knot.tube, ASCII.knot.tubular, ASCII.knot.radial, ASCII.knot.p, ASCII.knot.q);
  const atlasCanvas = document.createElement('canvas');
  const { distance: D, fovDeg, near, far } = ASCII.camera;
  const fov = (fovDeg * Math.PI) / 180;

  let res = setup();
  let W = 1;
  let H = 1;
  let dpr = 1;
  let cssW = 1;
  let cssH = 1;
  let cw = 8;
  let ch = 14;
  let proj: Mat4 = identity();
  let fit = fitKnot(1, 1);
  let lost = false;
  let dead = false;

  function fail() {
    if (dead) return;
    dead = true;
    onFail?.();
  }

  function compile(type: number, src: string): WebGLShader {
    const s = gl.createShader(type)!;
    gl.shaderSource(s, src);
    gl.compileShader(s);
    if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) throw new Error('shader');
    return s;
  }

  function program(vs: string, fs: string): Program {
    const p = gl.createProgram()!;
    const v = compile(gl.VERTEX_SHADER, vs);
    const f = compile(gl.FRAGMENT_SHADER, fs);
    gl.attachShader(p, v);
    gl.attachShader(p, f);
    gl.linkProgram(p);
    gl.deleteShader(v);
    gl.deleteShader(f);
    if (!gl.getProgramParameter(p, gl.LINK_STATUS)) throw new Error('link');
    const u: Program['u'] = {};
    const n = gl.getProgramParameter(p, gl.ACTIVE_UNIFORMS) as number;
    for (let i = 0; i < n; i++) {
      const info = gl.getActiveUniform(p, i)!;
      u[info.name] = gl.getUniformLocation(p, info.name);
    }
    return { p, u };
  }

  function texture(): WebGLTexture {
    const t = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D, t);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    return t;
  }

  function target(): Target {
    return { tex: texture(), rb: gl.createRenderbuffer()!, fb: gl.createFramebuffer()!, w: 0, h: 0 };
  }

  function buffer(kind: number, data: BufferSource): WebGLBuffer {
    const b = gl.createBuffer()!;
    gl.bindBuffer(kind, b);
    gl.bufferData(kind, data, gl.STATIC_DRAW);
    return b;
  }

  function setup() {
    const scene = program(SCENE_VS, SCENE_FS);
    const ascii = program(QUAD_VS, ASCII_FS);
    return {
      scene,
      ascii,
      aPos: gl.getAttribLocation(scene.p, 'aPos'),
      aNrm: gl.getAttribLocation(scene.p, 'aNrm'),
      qPos: gl.getAttribLocation(ascii.p, 'aPos'),
      pos: buffer(gl.ARRAY_BUFFER, knot.positions),
      nrm: buffer(gl.ARRAY_BUFFER, knot.normals),
      idx: buffer(gl.ELEMENT_ARRAY_BUFFER, knot.indices),
      quad: buffer(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1])),
      lum: target(),
      nd: target(),
      atlas: texture(),
    };
  }

  function sizeTarget(t: Target, w: number, h: number) {
    t.w = w;
    t.h = h;
    gl.bindTexture(gl.TEXTURE_2D, t.tex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
    gl.bindRenderbuffer(gl.RENDERBUFFER, t.rb);
    gl.renderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT16, w, h);
    gl.bindFramebuffer(gl.FRAMEBUFFER, t.fb);
    gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, t.tex, 0);
    gl.framebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, t.rb);
    const ok = gl.checkFramebufferStatus(gl.FRAMEBUFFER) === gl.FRAMEBUFFER_COMPLETE;
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    if (!ok) throw new Error('framebuffer');
  }

  function buildAtlas() {
    const fontPx = ASCII.cellPx * dpr;
    const c = atlasCanvas.getContext('2d');
    if (!c) throw new Error('2d');
    c.font = ASCII.font.replace('{px}', String(fontPx));
    cw = Math.max(4, Math.round(c.measureText('M').width));
    ch = Math.max(6, Math.round(ASCII.cellPx * ASCII.cellHeightRatio * dpr));
    atlasCanvas.width = cw * GLYPHS.length;
    atlasCanvas.height = ch;
    // Resizing a canvas resets its state, so set the font again.
    c.font = ASCII.font.replace('{px}', String(fontPx));
    c.clearRect(0, 0, atlasCanvas.width, ch);
    c.fillStyle = '#fff';
    c.textAlign = 'center';
    c.textBaseline = 'middle';
    GLYPHS.forEach((g, i) => c.fillText(g, i * cw + cw / 2, ch / 2 + ch * 0.04));
    gl.bindTexture(gl.TEXTURE_2D, res.atlas);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, atlasCanvas);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, false);
    const { cols, rows } = gridSize(W, H, cw, ch);
    sizeTarget(res.lum, cols * ASCII.supersample, rows * ASCII.supersample);
    sizeTarget(res.nd, cols * ASCII.supersample, rows * ASCII.supersample);
  }

  function applySize() {
    W = canvas.width = Math.max(1, Math.round(cssW * dpr));
    H = canvas.height = Math.max(1, Math.round(cssH * dpr));
    proj = perspective(fov, cssW / Math.max(cssH, 1), near, far);
    fit = fitKnot(cssW, cssH);
    buildAtlas();
  }

  function drawKnot(t: Target, mode: number, clear: [number, number, number, number], mv: Mat4, nm: Float32Array) {
    const { scene, aPos, aNrm } = res;
    gl.bindFramebuffer(gl.FRAMEBUFFER, t.fb);
    gl.viewport(0, 0, t.w, t.h);
    gl.clearColor(clear[0], clear[1], clear[2], clear[3]);
    gl.clearDepth(1);
    gl.enable(gl.DEPTH_TEST);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    gl.useProgram(scene.p);
    gl.uniformMatrix4fv(scene.u.uProj, false, proj);
    gl.uniformMatrix4fv(scene.u.uMV, false, mv);
    gl.uniformMatrix3fv(scene.u.uNM, false, nm);
    gl.uniform1f(scene.u.uMode, mode);
    gl.uniform1f(scene.u.uNear, D - 3);
    gl.uniform1f(scene.u.uFar, D + 3);
    gl.bindBuffer(gl.ARRAY_BUFFER, res.pos);
    gl.enableVertexAttribArray(aPos);
    gl.vertexAttribPointer(aPos, 3, gl.FLOAT, false, 0, 0);
    gl.bindBuffer(gl.ARRAY_BUFFER, res.nrm);
    gl.enableVertexAttribArray(aNrm);
    gl.vertexAttribPointer(aNrm, 3, gl.FLOAT, false, 0, 0);
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, res.idx);
    gl.drawElements(gl.TRIANGLES, knot.indices.length, gl.UNSIGNED_SHORT, 0);
    gl.disableVertexAttribArray(aPos);
    gl.disableVertexAttribArray(aNrm);
  }

  function frame(s: FrameState) {
    if (lost || dead || gl.isContextLost()) return;
    const m = ASCII.motion;
    const model = multiply(
      multiply(multiply(translate(fit.x, fit.y, 0), scale(fit.scale)), multiply(rotateX(s.tiltX), rotateY(s.yaw + s.tiltY))),
      multiply(rotateX(s.t * m.rotX), rotateY(s.t * m.rotY)),
    );
    const mv = multiply(translate(0, 0, -D), model);
    const nm = normalMatrix3(model);
    drawKnot(res.lum, 0, [0, 0, 0, 0], mv, nm);
    drawKnot(res.nd, 1, [0.5, 0.5, 1, 1], mv, nm);

    const { ascii, qPos } = res;
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    gl.viewport(0, 0, W, H);
    gl.disable(gl.DEPTH_TEST);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.useProgram(ascii.p);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, res.lum.tex);
    gl.uniform1i(ascii.u.uLum, 0);
    gl.activeTexture(gl.TEXTURE1);
    gl.bindTexture(gl.TEXTURE_2D, res.nd.tex);
    gl.uniform1i(ascii.u.uNrm, 1);
    gl.activeTexture(gl.TEXTURE2);
    gl.bindTexture(gl.TEXTURE_2D, res.atlas);
    gl.uniform1i(ascii.u.uAtlas, 2);
    gl.uniform2f(ascii.u.uRes, W, H);
    gl.uniform2f(ascii.u.uCell, cw, ch);
    gl.uniform1f(ascii.u.uRampN, RAMP.length);
    gl.uniform1f(ascii.u.uGlyphN, GLYPHS.length);
    gl.uniform1f(ascii.u.uTime, s.t);
    gl.uniform1f(ascii.u.uIntro, s.intro);
    gl.uniform1f(ascii.u.uSweep, s.sweep);
    if (s.pointer) {
      gl.uniform2f(ascii.u.uPointer, s.pointer[0] * dpr, H - s.pointer[1] * dpr);
      gl.uniform1f(ascii.u.uLens, ASCII.lensPx * dpr);
    } else {
      gl.uniform2f(ascii.u.uPointer, -1e5, -1e5);
      gl.uniform1f(ascii.u.uLens, 0);
    }
    gl.uniform3fv(ascii.u.uInkDark, rgb(ASCII.ink.bright));
    gl.uniform3fv(ascii.u.uInkLight, rgb(ASCII.ink.dim));
    gl.uniform3fv(ascii.u.uDust, rgb(ASCII.ink.dust));
    gl.bindBuffer(gl.ARRAY_BUFFER, res.quad);
    gl.enableVertexAttribArray(qPos);
    gl.vertexAttribPointer(qPos, 2, gl.FLOAT, false, 0, 0);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
    gl.disableVertexAttribArray(qPos);
  }

  function release() {
    for (const prog of [res.scene, res.ascii]) gl.deleteProgram(prog.p);
    for (const b of [res.pos, res.nrm, res.idx, res.quad]) gl.deleteBuffer(b);
    for (const t of [res.lum, res.nd]) {
      gl.deleteTexture(t.tex);
      gl.deleteRenderbuffer(t.rb);
      gl.deleteFramebuffer(t.fb);
    }
    gl.deleteTexture(res.atlas);
  }

  const onLost = (e: Event) => {
    e.preventDefault();
    lost = true;
  };
  const onRestored = () => {
    try {
      res = setup();
      applySize();
      lost = false;
    } catch {
      fail();
    }
  };
  canvas.addEventListener('webglcontextlost', onLost);
  canvas.addEventListener('webglcontextrestored', onRestored);

  return {
    resize(w, h, ratio) {
      cssW = Math.max(1, w);
      cssH = Math.max(1, h);
      dpr = Math.min(ASCII.dprCap, Math.max(1, ratio || 1));
      if (lost || dead) return;
      try {
        applySize();
      } catch {
        fail();
      }
    },
    frame(s) {
      try {
        frame(s);
      } catch {
        fail();
      }
    },
    dispose() {
      canvas.removeEventListener('webglcontextlost', onLost);
      canvas.removeEventListener('webglcontextrestored', onRestored);
      dead = true;
      if (!gl.isContextLost()) release();
    },
  };
}
