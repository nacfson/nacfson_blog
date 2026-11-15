export type KnotGeometry = { positions: Float32Array; normals: Float32Array; indices: Uint16Array };

type V3 = [number, number, number];
const sub = (a: V3, b: V3): V3 => [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
const add = (a: V3, b: V3): V3 => [a[0] + b[0], a[1] + b[1], a[2] + b[2]];
const cross = (a: V3, b: V3): V3 => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
const norm = (a: V3): V3 => {
  const l = Math.hypot(a[0], a[1], a[2]) || 1;
  return [a[0] / l, a[1] / l, a[2] / l];
};

/** (p, q) torus knot tube, built with the same parametrisation three.js uses. */
export function torusKnot(R: number, tube: number, tubular: number, radial: number, p: number, q: number): KnotGeometry {
  const ring = radial + 1;
  const positions = new Float32Array((tubular + 1) * ring * 3);
  const normals = new Float32Array(positions.length);
  const indices = new Uint16Array(tubular * radial * 6);

  const at = (u: number): V3 => {
    const qu = (q / p) * u;
    const cs = Math.cos(qu);
    return [R * (2 + cs) * 0.5 * Math.cos(u), R * (2 + cs) * 0.5 * Math.sin(u), R * Math.sin(qu) * 0.5];
  };

  let v = 0;
  for (let i = 0; i <= tubular; i++) {
    const u = (i / tubular) * p * Math.PI * 2;
    const p1 = at(u);
    const p2 = at(u + 0.01);
    const t = sub(p2, p1);
    const b = norm(cross(t, add(p2, p1)));
    const n = norm(cross(b, t));
    for (let j = 0; j <= radial; j++) {
      const a = (j / radial) * Math.PI * 2;
      const cx = -tube * Math.cos(a);
      const cy = tube * Math.sin(a);
      const x = p1[0] + cx * n[0] + cy * b[0];
      const y = p1[1] + cx * n[1] + cy * b[1];
      const z = p1[2] + cx * n[2] + cy * b[2];
      const nn = norm([x - p1[0], y - p1[1], z - p1[2]]);
      positions.set([x, y, z], v * 3);
      normals.set(nn, v * 3);
      v++;
    }
  }

  let k = 0;
  for (let j = 1; j <= tubular; j++)
    for (let i = 1; i <= radial; i++) {
      const a = ring * (j - 1) + (i - 1);
      const b = ring * j + (i - 1);
      const c = ring * j + i;
      const d = ring * (j - 1) + i;
      indices.set([a, b, d, b, c, d], k);
      k += 6;
    }

  return { positions, normals, indices };
}
