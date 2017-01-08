import time
import random

class Field:
    def __init__(self, s=[], w=0, h=0):
        self.s = s
        self.w = w
        self.h = h
    
    def Set(f, x, y, b):
        f.s[y][x] = b
    
    def Alive(f, x, y):
        x += f.w
        x %= f.w
        y += f.h
        y %= f.h
        return f.s[y][x]
    
    def Next(f, x, y):
        alive = 0
        i = -1
        while i<=1:
            j = -1
            while j<=1:
                if (j!=0 or i!=0) and f.Alive((x+i), (y+j)):
                    alive += 1
                
                j += 1
            
            i += 1
        
        return alive==3 or alive==2 and f.Alive(x, y)
    

class Life:
    def __init__(self, a=None, b=None, w=0, h=0):
        self.a = a
        self.b = b
        self.w = w
        self.h = h
    
    def Step(l):
        y = 0
        while y<l.h:
            x = 0
            while x<l.w:
                l.b.Set(x, y, l.a.Next(x, y))
                x += 1
            
            y += 1
        
        l.a, l.b = (l.b), (l.a)
    
    def String(l):
        buf = ""
        y = 0
        while y<l.h:
            x = 0
            while x<l.w:
                b = ' '
                if l.a.Alive(x, y):
                    b = '*'
                
                buf += b
                x += 1
            
            buf += '\n'
            y += 1
        
        return buf
    

def NewField(w, h):
    s = [[]]*h
    for i in range(len(s)):
        s[i] = [False]*w
    
    return Field(s=s, w=w, h=h)

def NewLife(w, h):
    a = NewField(w, h)
    i = 0
    while i<w*h//4:
        a.Set(random.randrange(w), random.randrange(h), True)
        i += 1
    
    return Life(a=a, b=NewField(w, h), w=w, h=h)

def main():
    l = NewLife(40, 15)
    i = 0
    while i<300:
        l.Step()
        print("\n\n", l.String())
        time.sleep(1/30)
        i += 1
    
if __name__ == "__main__":
    main()