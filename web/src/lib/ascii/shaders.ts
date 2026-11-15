import { ASCII } from './constants';

/** Format a number as a GLSL ES 1.0 float literal. */
const f = (n: number) => n.toFixed(2);
const vec3 = (v: readonly number[]) => `vec3(${v.map(f).join(',')})`;

export const SCENE_VS = `attribute vec3 aPos; attribute vec3 aNrm;
uniform mat4 uProj; uniform mat4 uMV; uniform mat3 uNM;
varying vec3 vN; varying vec3 vP;
void main(){ vec4 mv=uMV*vec4(aPos,1.0); vP=mv.xyz; vN=uNM*aNrm; gl_Position=uProj*mv; }`;

const lightTerms = ASCII.lights.map((l) => `lt(n,v,normalize(${vec3(l.dir)}),${f(l.k)})`).join('+');

/** Pass 1 (uMode 0): grey light with coverage in alpha. Pass 2 (uMode 1): view normal + linear depth. */
export const SCENE_FS = `precision highp float;
uniform float uMode; uniform float uNear; uniform float uFar;
varying vec3 vN; varying vec3 vP;
float lt(vec3 n,vec3 v,vec3 l,float k){ vec3 h=normalize(l+v); return k*(max(dot(n,l),0.0)+${f(ASCII.light.specWeight)}*pow(max(dot(n,h),0.0),${ASCII.light.specPower.toFixed(1)})); }
void main(){
  vec3 n=normalize(vN);
  if(uMode>0.5){ gl_FragColor=vec4(n*0.5+0.5,clamp((-vP.z-uNear)/(uFar-uNear),0.0,1.0)); return; }
  vec3 v=normalize(-vP);
  float c=${f(ASCII.light.ambient)}+${lightTerms};
  gl_FragColor=vec4(vec3(c),1.0);
}`;

export const QUAD_VS = `attribute vec2 aPos; varying vec2 vUv;
void main(){ vUv=aPos*0.5+0.5; gl_Position=vec4(aPos,0.0,1.0); }`;

/** Pass 3: one glyph per cell, chosen from light, edges, intro and lens state. Premultiplied output. */
export const ASCII_FS = `precision highp float;
uniform sampler2D uLum; uniform sampler2D uNrm; uniform sampler2D uAtlas;
uniform vec2 uRes; uniform vec2 uCell;
uniform float uRampN; uniform float uGlyphN;
uniform float uTime; uniform float uIntro; uniform float uSweep;
uniform vec2 uPointer; uniform float uLens;
uniform vec3 uInkDark; uniform vec3 uInkLight; uniform vec3 uDust;
varying vec2 vUv;
#define PI 3.14159265
float hash(vec2 p){ return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453); }
void main(){
  vec2 frag=vUv*uRes;
  vec2 cc=(floor(frag/uCell)+0.5)*uCell;
  float lens=(uLens>0.0 && length(cc-uPointer)<uLens)?1.0:0.0;
  vec2 cs=mix(uCell,uCell*0.5,lens);
  vec2 cell=floor(frag/cs);
  vec2 center=(cell+0.5)*cs;
  vec2 suv=center/uRes;
  vec4 L=texture2D(uLum,suv);
  float cov=L.a;
  float lum=pow(clamp(L.r,0.0,1.0),${f(ASCII.lum.gamma)});
  float sp=center.x/uRes.x*0.85+center.y/uRes.y*0.15;
  float sweep=uSweep>-0.5?smoothstep(${f(ASCII.sweep.halfWidth)},0.0,abs(sp-uSweep)):0.0;
  lum=clamp(lum+sweep*${f(ASCII.sweep.boost)}*cov,0.0,1.0);
  vec2 px=cs/uRes;
  vec4 nR=texture2D(uNrm,suv+vec2(px.x,0.0));
  vec4 nL=texture2D(uNrm,suv-vec2(px.x,0.0));
  vec4 nU=texture2D(uNrm,suv+vec2(0.0,px.y));
  vec4 nD=texture2D(uNrm,suv-vec2(0.0,px.y));
  vec2 dg=vec2(nR.a-nL.a,nU.a-nD.a);
  vec2 ng=vec2(dot(nR.rgb-nL.rgb,vec3(1.0)),dot(nU.rgb-nD.rgb,vec3(1.0)));
  float nEdge=length(nR.rgb-nL.rgb)+length(nU.rgb-nD.rgb);
  float dEdge=abs(dg.x)+abs(dg.y);
  float gi;
  vec3 ink=mix(uInkLight,uInkDark,smoothstep(0.05,0.9,lum));
  if(cov>0.5 && (dEdge>${f(ASCII.edge.depth)} || nEdge>${f(ASCII.edge.normal)})){
    vec2 dir=length(dg)>0.02?dg:ng;
    float a=mod(atan(dir.y,dir.x)+PI,PI);
    float k=(a<PI/8.0 || a>=7.0*PI/8.0)?0.0:(a<3.0*PI/8.0?3.0:(a<5.0*PI/8.0?2.0:1.0));
    gi=uRampN+k; ink=uInkDark;
  } else {
    gi=floor(clamp(lum*cov,0.0,0.999)*uRampN);
    if(cov>0.5) gi=max(gi,1.0);
  }
  float th=hash(cell*0.73+3.1)*0.65+(center.x/uRes.x)*0.35;
  if(uIntro<th){
    float r=hash(cell+floor(uTime*22.0));
    if(cov>0.02){ gi=1.0+floor(r*(uRampN-1.0)); ink=mix(uInkLight,uInkDark,0.35); }
    else if(r>0.985){ gi=1.0+floor(hash(cell+7.0)*(uRampN-1.0)); ink=uInkLight; }
  }
  if(gi<0.5 && cov<0.02){
    float tw=hash(cell*1.7+floor(uTime*0.6+hash(cell)*10.0));
    if(tw>0.994){ gi=1.0; ink=uDust; }
  }
  if(gi<0.5){ gl_FragColor=vec4(0.0); return; }
  vec2 inCell=clamp((frag-cell*cs)/cs,0.02,0.98);
  float g=texture2D(uAtlas,vec2((gi+inCell.x)/uGlyphN,inCell.y)).a;
  gl_FragColor=vec4(ink*g,g);
}`;
